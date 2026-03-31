package openapi3filter_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"
)

const spec31GapsJSON = `{
  "openapi": "3.1.0",
  "info": { "title": "OAS 3.1 Gap Tests", "version": "1.0.0" },
  "servers": [{ "url": "/api/v31gap" }],
  "paths": {
    "/widget": {
      "$ref": "#/components/pathItems/WidgetPath"
    }
  },
  "components": {
    "pathItems": {
      "WidgetPath": {
        "post": {
          "operationId": "createWidget",
          "requestBody": {
            "required": true,
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Widget" }
              }
            }
          },
          "responses": { "200": { "description": "ok" } }
        }
      }
    },
    "schemas": {
      "Widget": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": { "type": "string" }
        }
      }
    }
  }
}`

// TestComponentsPathItemsResolution verifies that a path item defined via
// $ref to components.pathItems is properly resolved and routable.
func TestComponentsPathItemsResolution(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(spec31GapsJSON))
	require.NoError(t, err, "LoadFromData should succeed")

	// Verify the path item was resolved: /widget should have a Post operation
	pathItem := doc.Paths.Value("/widget")
	require.NotNil(t, pathItem, "doc.Paths must contain /widget")
	t.Logf("/widget pathItem.Ref = %q", pathItem.Ref)
	t.Logf("/widget pathItem.Post = %v", pathItem.Post)
	require.NotNil(t, pathItem.Post, "/widget PathItem must have Post operation after ref resolution")
	require.Equal(t, "createWidget", pathItem.Post.OperationID)

	// Validate the document
	err = doc.Validate(context.Background())
	require.NoError(t, err, "doc.Validate should succeed")

	// Build router
	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err, "NewRouter should succeed")

	// Valid request: POST /api/v31gap/widget with correct body
	validBody := `{"name": "foo"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v31gap/widget", bytes.NewBufferString(validBody))
	req.Header.Set("Content-Type", "application/json")

	route, pathParams, err := router.FindRoute(req)
	require.NoError(t, err, "FindRoute should find POST /api/v31gap/widget")
	t.Logf("route.Path = %q, route.Method = %q", route.Path, route.Method)
	require.NotNil(t, route.Operation, "route.Operation must not be nil")
	t.Logf("route.Operation.OperationID = %q", route.Operation.OperationID)

	input := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
	}
	err = openapi3filter.ValidateRequest(context.Background(), input)
	require.NoError(t, err, "valid request should pass validation")

	// Invalid request: POST /api/v31gap/widget with wrong body (missing required 'name')
	invalidBody := `{"notaname": "foo"}`
	req2, _ := http.NewRequest(http.MethodPost, "/api/v31gap/widget", bytes.NewBufferString(invalidBody))
	req2.Header.Set("Content-Type", "application/json")

	route2, pathParams2, err := router.FindRoute(req2)
	require.NoError(t, err, "FindRoute should find POST /api/v31gap/widget for invalid body request")

	input2 := &openapi3filter.RequestValidationInput{
		Request:    req2,
		PathParams: pathParams2,
		Route:      route2,
	}
	err = openapi3filter.ValidateRequest(context.Background(), input2)
	require.Error(t, err, "invalid body (missing required 'name') should fail validation")
	t.Logf("validation error (expected): %v", err)
}
