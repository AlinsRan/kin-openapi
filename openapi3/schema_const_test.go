package openapi3

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaConst_BuiltInValidator(t *testing.T) {
	t.Run("string const", func(t *testing.T) {
		schema := &Schema{
			Const: "production",
		}

		err := schema.VisitJSON("production")
		require.NoError(t, err)

		err = schema.VisitJSON("development")
		require.Error(t, err)
		require.ErrorContains(t, err, "const")
	})

	t.Run("number const", func(t *testing.T) {
		schema := &Schema{
			Const: float64(42),
		}

		err := schema.VisitJSON(float64(42))
		require.NoError(t, err)

		err = schema.VisitJSON(float64(43))
		require.Error(t, err)
	})

	t.Run("boolean const", func(t *testing.T) {
		schema := &Schema{
			Const: true,
		}

		err := schema.VisitJSON(true)
		require.NoError(t, err)

		err = schema.VisitJSON(false)
		require.Error(t, err)
	})

	t.Run("null const", func(t *testing.T) {
		schema := &Schema{
			Type:       &Types{"null"},
			Const:      nil,
			ConstIsSet: true,
		}

		// const: null — only nil is valid
		err := schema.VisitJSON(nil)
		require.NoError(t, err)

		err = schema.VisitJSON("not null")
		require.Error(t, err)
	})

	t.Run("null const without type permits nil in built-in validator", func(t *testing.T) {
		schema := &Schema{
			Const:      nil,
			ConstIsSet: true,
		}

		err := schema.VisitJSON(nil)
		require.NoError(t, err)
	})

	t.Run("null const without type rejects non-nil", func(t *testing.T) {
		schema := &Schema{
			Const:      nil,
			ConstIsSet: true,
		}

		err := schema.VisitJSON("not null")
		require.Error(t, err)
		require.ErrorContains(t, err, "const")
	})

	t.Run("object const", func(t *testing.T) {
		schema := &Schema{
			Const: map[string]any{"key": "value"},
		}

		err := schema.VisitJSON(map[string]any{"key": "value"})
		require.NoError(t, err)

		err = schema.VisitJSON(map[string]any{"key": "other"})
		require.Error(t, err)
	})

	t.Run("const with type constraint", func(t *testing.T) {
		schema := &Schema{
			Type:  &Types{"string"},
			Const: "fixed",
		}

		err := schema.VisitJSON("fixed")
		require.NoError(t, err)

		err = schema.VisitJSON("other")
		require.Error(t, err)
	})

	t.Run("const with multiError", func(t *testing.T) {
		schema := &Schema{
			Type:  &Types{"string"},
			Const: "fixed",
		}

		err := schema.VisitJSON("other", MultiErrors())
		require.Error(t, err)
	})
}
