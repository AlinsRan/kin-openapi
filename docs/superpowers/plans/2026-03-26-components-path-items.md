# components.pathItems Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `components.pathItems` (OpenAPI 3.1 §4.8.14) support with loading, validation, $ref resolution, routing, and request/response validation.

**Architecture:** `Components` gains a `PathItems map[string]*PathItem` field. The loader resolves `$ref: '#/components/pathItems/...'` by drilling through `Components.PathItems` in `resolveComponent`. Both routers dereference a PathItem's `.Ref` to get the resolved `*PathItem` value (already done by the loader) before building routes, so no new router logic is strictly needed — the resolved value is already in place. A test fixture and integration tests cover the full path.

**Tech Stack:** Go 1.21, `encoding/json`, `gopkg.in/yaml.v3`, `github.com/gorilla/mux`, `github.com/getkin/kin-openapi/openapi3`

---

## File Map

| File | Change |
|------|--------|
| `openapi3/components.go` | Add `PathItems` field; update `MarshalYAML`, `UnmarshalJSON`, `Validate()` |
| `openapi3/loader.go` | In `ResolveRefsIn`: iterate `components.PathItems` and call `resolvePathItemRef`. The `resolveComponent` drill already works via reflection — no extra case needed. |
| `openapi3/testdata/components-path-items.yml` | New 3.1 fixture with `components.pathItems` and a path referencing it |
| `openapi3/loader_test.go` | New test: load + validate fixture; assert `Components.PathItems["MyItem"]` resolves |
| `routers/gorillamux/router_test.go` | New test: NewRouter on fixture doc, FindRoute succeeds |
| `openapi3filter/` (existing tests) | New test: ValidateRequest on a route resolved from `components.pathItems` $ref |

---

## Task 1: Add `PathItems` field to `Components`

**Files:**
- Modify: `openapi3/components.go`

- [ ] **Step 1: Add type alias and field**

In `openapi3/components.go`, add `PathItems` map type and field:

```go
// In the type block (line ~12-22), add:
PathItems map[string]*PathItem

// In Components struct (line ~26), add after Callbacks:
PathItems map[string]*PathItem `json:"pathItems,omitempty" yaml:"pathItems,omitempty"`
```

> **Note:** No separate named type needed — `map[string]*PathItem` used directly (same as `doc.Webhooks`).

- [ ] **Step 2: Update `MarshalYAML`**

In `MarshalYAML` (around line 56), after the `callbacks` block and before `return m, nil`, add:

```go
if x := components.PathItems; len(x) != 0 {
    m["pathItems"] = x
}
```

Also bump the capacity hint from `9` to `10`.

- [ ] **Step 3: Update `UnmarshalJSON`**

In `UnmarshalJSON` (around line 91-113), add `delete(x.Extensions, "pathItems")` alongside the other deletes.

- [ ] **Step 4: Update `Validate()`**

In `Validate()` (around line 239-252 after callbacks), add:

```go
pathItems := make([]string, 0, len(components.PathItems))
for name := range components.PathItems {
    pathItems = append(pathItems, name)
}
sort.Strings(pathItems)
for _, k := range pathItems {
    v := components.PathItems[k]
    if v == nil {
        return fmt.Errorf("pathItem %q: value is nil", k)
    }
    if err = ValidateIdentifier(k); err != nil {
        return fmt.Errorf("pathItem %q: %w", k, err)
    }
    if err = v.Validate(ctx); err != nil {
        return fmt.Errorf("pathItem %q: %w", k, err)
    }
}
```

- [ ] **Step 5: Verify it compiles**

```bash
go build ./openapi3/...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add openapi3/components.go
git commit -m "feat: add PathItems field to Components for OpenAPI 3.1"
```

---

## Task 2: Loader — resolve `components.PathItems` refs

**Files:**
- Modify: `openapi3/loader.go`

**Context:** `resolveComponent` already uses `drillIntoField` (reflection) to drill into struct fields by their JSON tag name. Since we named the field `PathItems` with JSON tag `pathItems`, a ref like `#/components/pathItems/MyItem` will drill: `components` → `Components` struct → field `PathItems` (via reflection on json tag) → map key `MyItem`. No new case in `resolveComponent` is needed.

What IS needed: in `ResolveRefsIn`, after resolving `components.Callbacks`, add iteration over `components.PathItems` to call `resolvePathItemRef`.

- [ ] **Step 1: Add PathItems resolution to `ResolveRefsIn`**

In `loader.go` around line 245-251 (after the Callbacks block, before the closing `}`), add:

```go
for _, name := range componentNames(components.PathItems) {
    pathItem := components.PathItems[name]
    if pathItem == nil {
        continue
    }
    if err = loader.resolvePathItemRef(doc, pathItem, location); err != nil {
        return
    }
}
```

- [ ] **Step 2: Confirm reflection drill works for `pathItems`**

`drillIntoField` (loader.go ~line 561) matches struct fields by their **yaml tag**, not the Go field name. Since the new field will have `yaml:"pathItems,omitempty"`, the drill for `#/components/pathItems/MyItem` will resolve correctly without any special case. Run the following to confirm the tag approach:

```bash
grep -n "yaml\|drillIntoField" openapi3/loader.go | grep -A3 "drillIntoField"
```

Expected: you'll see yaml tag matching. No special `case *Components:` is needed.

- [ ] **Step 3: Verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add openapi3/loader.go
git commit -m "feat: resolve components.pathItems refs in loader"
```

---

## Task 3: Write test fixture and loader test

**Files:**
- Create: `openapi3/testdata/components-path-items.yml`
- Modify: `openapi3/loader_test.go` (add test)

- [ ] **Step 1: Write failing test first**

In `openapi3/loader_test.go`, add:

```go
func TestLoadComponentsPathItems(t *testing.T) {
    loader := NewLoader()
    doc, err := loader.LoadFromFile("testdata/components-path-items.yml")
    require.NoError(t, err)
    require.NotNil(t, doc.Components)
    require.NotNil(t, doc.Components.PathItems)

    item, ok := doc.Components.PathItems["MyItem"]
    require.True(t, ok, "components.pathItems should have 'MyItem'")
    require.NotNil(t, item)
    require.NotNil(t, item.Get, "MyItem should have a GET operation")

    // Test that paths/$ref was resolved
    pathItem := doc.Paths.Value("/things")
    require.NotNil(t, pathItem, "paths should have '/things'")
    // After ref resolution, the operations from MyItem are present
    require.NotNil(t, pathItem.Get, "/things should have a GET operation after ref resolution")

    // Validate
    err = doc.Validate(context.Background())
    require.NoError(t, err)
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./openapi3/... -run TestLoadComponentsPathItems -v
```

Expected: FAIL — fixture file does not exist yet.

- [ ] **Step 3: Create test fixture**

Create `openapi3/testdata/components-path-items.yml`:

```yaml
openapi: "3.1.0"
info:
  title: PathItems Test
  version: "1"
paths:
  /things:
    $ref: '#/components/pathItems/MyItem'
components:
  pathItems:
    MyItem:
      get:
        summary: Get things
        operationId: getThings
        responses:
          "200":
            description: OK
            content:
              application/json:
                schema:
                  type: object
                  properties:
                    id:
                      type: string
      post:
        summary: Create thing
        operationId: createThing
        requestBody:
          required: true
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
        responses:
          "201":
            description: Created
```

- [ ] **Step 4: Run test — should pass**

```bash
go test ./openapi3/... -run TestLoadComponentsPathItems -v
```

Expected: PASS.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add openapi3/testdata/components-path-items.yml openapi3/loader_test.go
git commit -m "test: add components.pathItems loader test and fixture"
```

---

## Task 4: Router test — routing through `components.pathItems` $ref

**Files:**
- Modify: `routers/gorillamux/router_test.go`

**Context:** The gorilla/mux router iterates `doc.Paths.InMatchingOrder()` and calls `doc.Paths.Value(path)` for each path, which returns the `*PathItem` stored in the Paths map. After `ResolveRefsIn` runs, a PathItem that had `$ref: '#/components/pathItems/MyItem'` has been **mutated in place** — its fields are overwritten with the resolved values (see `loader.go:1354`: `*pathItem = resolved`). So the router naturally sees the resolved operations without any change.

- [ ] **Step 1: Write failing test**

In `routers/gorillamux/router_test.go`, add:

```go
func TestRouterComponentsPathItems(t *testing.T) {
    loader := openapi3.NewLoader()
    doc, err := loader.LoadFromFile("../../openapi3/testdata/components-path-items.yml")
    require.NoError(t, err)
    err = doc.Validate(context.Background())
    require.NoError(t, err)

    router, err := NewRouter(doc)
    require.NoError(t, err)

    // GET /things
    req, err := http.NewRequest(http.MethodGet, "/things", nil)
    require.NoError(t, err)
    route, pathParams, err := router.FindRoute(req)
    require.NoError(t, err)
    require.NotNil(t, route)
    assert.Equal(t, "/things", route.Path)
    assert.NotNil(t, route.Operation)
    assert.Equal(t, "getThings", route.Operation.OperationID)
    _ = pathParams
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./routers/gorillamux/... -run TestRouterComponentsPathItems -v
```

Expected: FAIL (if routing fails) or PASS (if loader already resolves in-place correctly). If it PASSES immediately, that's expected behavior — the resolver already mutated the PathItem in place.

- [ ] **Step 3: If test fails, investigate**

If `FindRoute` fails with path not found, the issue is in `pathItem.Operations()` returning empty — meaning the PathItem wasn't fully resolved. Check `loader.go`'s `resolvePathItemRef` — it calls `*pathItem = resolved` which should copy all fields. If the `Paths` map stores a pointer and the resolver got a copy, the fix is to ensure `doc.Paths` stores the pointer that the resolver dereferences into.

Inspect `paths.go` Map() return type and whether `Paths.Value()` returns the same pointer that `ResolveRefsIn` iterates over.

- [ ] **Step 4: Fix if needed and run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add routers/gorillamux/router_test.go
git commit -m "test: add router test for components.pathItems $ref routing"
```

---

## Task 5: openapi3filter test — request validation through `components.pathItems`

**Files:**
- Modify: `openapi3filter/req_resp_decoder_test.go` or `openapi3filter/validate_request_test.go`

- [ ] **Step 1: Write failing test**

Add to `openapi3filter/validate_request_test.go`:

```go
func TestValidateRequestComponentsPathItems(t *testing.T) {
    loader := openapi3.NewLoader()
    doc, err := loader.LoadFromFile("../openapi3/testdata/components-path-items.yml")
    require.NoError(t, err)
    err = doc.Validate(context.Background())
    require.NoError(t, err)

    router, err := gorillamux.NewRouter(doc)
    require.NoError(t, err)

    // Valid POST request
    body := `{"name": "widget"}`
    req, err := http.NewRequest(http.MethodPost, "/things", strings.NewReader(body))
    require.NoError(t, err)
    req.Header.Set("Content-Type", "application/json")

    route, pathParams, err := router.FindRoute(req)
    require.NoError(t, err)

    input := &RequestValidationInput{
        Request:    req,
        PathParams: pathParams,
        Route:      route,
    }
    err = ValidateRequest(context.Background(), input)
    require.NoError(t, err, "valid POST /things should pass validation")

    // Invalid POST request — missing required field (schema has no required, so add one)
    // Actually the fixture doesn't mark 'name' as required, so test a bad content type instead
    req2, err := http.NewRequest(http.MethodPost, "/things", strings.NewReader(`not json`))
    require.NoError(t, err)
    req2.Header.Set("Content-Type", "application/json")

    route2, pathParams2, err := router.FindRoute(req2)
    require.NoError(t, err)

    input2 := &RequestValidationInput{
        Request:    req2,
        PathParams: pathParams2,
        Route:      route2,
    }
    err = ValidateRequest(context.Background(), input2)
    require.Error(t, err, "invalid JSON body should fail validation")
}
```

- [ ] **Step 2: Run test to confirm behavior**

```bash
go test ./openapi3filter/... -run TestValidateRequestComponentsPathItems -v
```

Expected: PASS (if routing + validation already work end-to-end) or surface the actual failure for fixing.

- [ ] **Step 3: Fix any failures found**

If `ValidateRequest` fails to find the operation (returns nil), the issue is that `route.Operation` wasn't set properly. Trace through `FindRoute` → `route.Operation = route.Spec.Paths.Value(route.Path).GetOperation(route.Method)`. This requires the PathItem to be fully resolved.

- [ ] **Step 4: Run full test suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add openapi3filter/validate_request_test.go
git commit -m "test: validate request/response through components.pathItems ref"
```

---

## Task 6: Regenerate docs and push

**Files:**
- Modify: `.github/docs/openapi3.txt` (generated)

- [ ] **Step 1: Regenerate docs**

```bash
./docs.sh
```

Expected: `.github/docs/openapi3.txt` is updated with the new `PathItems` field.

- [ ] **Step 2: Verify diff is only the new field**

```bash
git diff .github/docs/openapi3.txt
```

Expected: only `PathItems` and related additions.

- [ ] **Step 3: Commit**

```bash
git add .github/docs/openapi3.txt
git commit -m "chore: regenerate docs for components.PathItems field"
```

- [ ] **Step 4: Push to fork**

```bash
git push fork feat/openapi-3.1-support
```

---

## Verification checklist

Before claiming completion:

- [ ] `go test ./...` passes with 0 failures
- [ ] `go vet ./...` passes with 0 warnings  
- [ ] `./docs.sh && git diff --exit-code` passes (no uncommitted doc changes)
- [ ] `git push fork feat/openapi-3.1-support` succeeds

---

## Edge cases to keep in mind

1. **`resolveComponent` reflection drill**: The function uses `drillIntoField` which relies on reflection. Verify it matches JSON tag names (`pathItems`) not Go field names (`PathItems`). If it uses Go field names, add an explicit case in the `drill` switch:
   ```go
   case *Components:
       cursor = drillIntoField(c, pathPart) // or handle manually
   ```

2. **Pointer vs value semantics**: `resolvePathItemRef` does `*pathItem = resolved` which mutates the PathItem in place. Since `doc.Paths` stores `*PathItem` pointers, and the loader iterates those same pointers, the mutation is visible. But `components.PathItems` stores `*PathItem` as values in the map — the loader iterates the map and resolves refs inside each PathItem. A ref like `$ref: '#/components/pathItems/MyItem'` in `doc.Paths` points to a DIFFERENT `*PathItem` in the paths map — that one gets resolved by `resolvePathItemRef` which calls `resolveComponent` to drill into `components.PathItems["MyItem"]` and copy the resolved PathItem back.

3. **Circular refs**: The existing `visitedPathItemRefs` / `shouldVisitRef` mechanism handles cycles — no special handling needed.

4. **3.0 compatibility**: `components.pathItems` is a 3.1-only feature. The `Validate()` addition doesn't need a version guard since it simply iterates an empty map on 3.0 docs. No harm.
