package openapi3filter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"
)

// TestComponentsPathItemsValidateRequestSimulateMainGo simulates the exact logic
// in lua-resty-openapi-validate/src/main.go's validateRequest function.
func TestComponentsPathItemsValidateRequestSimulateMainGo(t *testing.T) {
	openAPI := spec31GapsJSON

	// Simulate createRouter (no doc.Validate call, just like main.go)
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(openAPI))
	require.NoError(t, err)

	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)

	// Simulate validateRequest for an invalid body
	method := "POST"
	path := "/api/v31gap/widget"  // This is ctx.var.request_uri
	body := `{"notaname": "foo"}` // invalid: missing required "name"

	// Parse headers exactly as main.go does
	headersJSON := `{"content-type":"application/json"}`
	h := make(map[string]interface{})
	err = json.Unmarshal([]byte(headersJSON), &h)
	require.NoError(t, err)
	headers := make(map[string][]string)
	for k, v := range h {
		switch val := v.(type) {
		case string:
			headers[k] = []string{val}
		case []interface{}:
			for _, item := range val {
				headers[k] = append(headers[k], item.(string))
			}
		}
	}

	httpReq, _ := http.NewRequest(method, path, strings.NewReader(body))
	httpReq.Header = headers

	route, pathParams, err := router.FindRoute(httpReq)
	require.NoError(t, err, "FindRoute must succeed for POST /api/v31gap/widget")
	t.Logf("route.Path=%q route.Method=%q", route.Path, route.Method)
	require.NotNil(t, route.Operation, "route.Operation must not be nil")
	t.Logf("route.Operation.OperationID=%q", route.Operation.OperationID)

	opts := &openapi3filter.Options{
		ExcludeRequestBody: false,
		MultiError:         true,
	}
	err = openapi3filter.ValidateRequest(context.Background(), &openapi3filter.RequestValidationInput{
		Request:    httpReq,
		PathParams: pathParams,
		Route:      route,
		Options:    opts,
	})
	require.Error(t, err, "invalid body should fail validation")
	t.Logf("ValidateRequest error: %v", err)

	// Simulate what main.go does with the error - build retErr string
	retErr := ""
	switch e := err.(type) {
	case openapi3.MultiError:
		for _, sub := range e {
			switch se := sub.(type) {
			case *openapi3filter.RequestError:
				if se.Parameter != nil {
					// header/path/query errors
				}
				if se.RequestBody != nil {
					retErr = retErr + "\n" + se.Error()
				}
			case *openapi3.SchemaError:
				retErr = retErr + se.Error()
			}
		}
	}

	t.Logf("retErr=%q", retErr)
	require.NotEmpty(t, retErr, "retErr must not be empty - Lua treats empty string as success!")
}
