package openapi3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type securitySchemeExample struct {
	title string
	raw   []byte
	valid bool
}

func TestSecuritySchemaExample(t *testing.T) {
	for _, example := range securitySchemeExamples {
		t.Run(example.title, func(t *testing.T) {
			ss := &SecurityScheme{}
			err := ss.UnmarshalJSON(example.raw)
			require.NoError(t, err)

			err = ss.Validate(context.Background())
			if example.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// from https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.3.md#fixed-fields-23
var securitySchemeExamples = []securitySchemeExample{
	{
		title: "Basic Authentication Sample",
		raw: []byte(`{
  "type": "http",
  "scheme": "basic"
}`),
		valid: true,
	},

	{
		title: "Negotiate Authentication Sample",
		raw: []byte(`{
  "type": "http",
  "scheme": "negotiate"
}`),
		valid: true,
	},

	{
		title: "Unknown http Authentication Sample",
		raw: []byte(`{
  "type": "http",
  "scheme": "notvalid"
}`),
		valid: false,
	},

	{
		title: "API Key Sample",
		raw: []byte(`{
  "type": "apiKey",
  "name": "api_key",
  "in": "header"
}`),
		valid: true,
	},

	{
		title: "apiKey with bearerFormat",
		raw: []byte(`{
  "type": "apiKey",
  "in": "header",
  "name": "X-API-KEY",
  "bearerFormat": "Arbitrary text"
}`),
		valid: false,
	},

	{
		title: "Bearer Sample with arbitrary format",
		raw: []byte(`{
  "type": "http",
  "scheme": "bearer",
  "bearerFormat": "Arbitrary text"
}`),
		valid: true,
	},

	{
		title: "Implicit OAuth2 Sample",
		raw: []byte(`{
  "type": "oauth2",
  "flows": {
    "implicit": {
      "authorizationUrl": "https://example.com/api/oauth/dialog",
      "scopes": {
        "write:pets": "modify pets in your account",
        "read:pets": "read your pets"
      }
    }
  }
}`),
		valid: true,
	},

	{
		title: "OAuth Flow Object Sample",
		raw: []byte(`{
  "type": "oauth2",
  "flows": {
    "implicit": {
      "authorizationUrl": "https://example.com/api/oauth/dialog",
      "scopes": {
        "write:pets": "modify pets in your account",
        "read:pets": "read your pets"
      }
    },
    "authorizationCode": {
      "authorizationUrl": "https://example.com/api/oauth/dialog",
      "tokenUrl": "https://example.com/api/oauth/token",
      "scopes": {
        "write:pets": "modify pets in your account",
        "read:pets": "read your pets"
      }
    }
  }
}`),
		valid: true,
	},

	{
		title: "OAuth Flow Object clientCredentials/password",
		raw: []byte(`{
  "type": "oauth2",
  "flows": {
    "clientCredentials": {
      "tokenUrl": "https://example.com/api/oauth/token",
      "scopes": {
        "write:pets": "modify pets in your account"
      }
    },
    "password": {
      "tokenUrl": "https://example.com/api/oauth/token",
      "scopes": {
        "read:pets": "read your pets"
      }
    }
  }
}`),
		valid: true,
	},

	{
		title: "Invalid Basic",
		raw: []byte(`{
  "type": "https",
  "scheme": "basic"
}`),
		valid: false,
	},

	{
		title: "Apikey Cookie",
		raw: []byte(`{
  "type": "apiKey",
	"in": "cookie",
	"name": "somecookie"
}`),
		valid: true,
	},

	{
		title: "OAuth Flow Object with no scopes",
		raw: []byte(`{
  "type": "oauth2",
  "flows": {
    "password": {
      "tokenUrl": "https://example.com/api/oauth/token"
    }
  }
}`),
		valid: false,
	},

	{
		title: "OAuth Flow Object with empty scopes",
		raw: []byte(`{
  "type": "oauth2",
  "flows": {
    "password": {
      "tokenUrl": "https://example.com/api/oauth/token",
      "scopes": {}
    }
  }
}`),
		valid: true,
	},

	{
		title: "OIDC Type With URL",
		raw: []byte(`{
  "type": "openIdConnect",
  "openIdConnectUrl": "https://example.com/.well-known/openid-configuration"
}`),
		valid: true,
	},

	{
		title: "OIDC Type Without URL",
		raw: []byte(`{
  "type": "openIdConnect",
  "openIdConnectUrl": ""
}`),
		valid: false,
	},
}

// TestMutualTLSVersionGating verifies that mutualTLS security scheme type is
// only accepted when validating an OpenAPI 3.1 document. When validated via
// SecurityScheme.Validate() directly (no version context), or within a 3.0
// document, it must return an error.
func TestMutualTLSVersionGating(t *testing.T) {
	mutualTLSScheme := &SecurityScheme{Type: "mutualTLS"}

	t.Run("mutualTLS rejected without version context", func(t *testing.T) {
		err := mutualTLSScheme.Validate(context.Background())
		require.ErrorContains(t, err, "mutualTLS")
	})

	t.Run("mutualTLS rejected in OpenAPI 3.0 context", func(t *testing.T) {
		ctx := WithValidationOptions(context.Background()) // no setOpenAPI31
		err := mutualTLSScheme.Validate(ctx)
		require.ErrorContains(t, err, "mutualTLS")
	})

	t.Run("mutualTLS accepted in OpenAPI 3.1 context", func(t *testing.T) {
		ctx := WithValidationOptions(context.Background(), setOpenAPI31())
		err := mutualTLSScheme.Validate(ctx)
		require.NoError(t, err)
	})
}
