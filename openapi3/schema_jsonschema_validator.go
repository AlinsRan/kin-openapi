package openapi3

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type jsonSchemaValidator struct {
	compiler *jsonschema.Compiler
	schema   *jsonschema.Schema
}

func newJSONSchemaValidator(schema *Schema) (*jsonSchemaValidator, error) {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	transformOpenAPIToJSONSchema(schemaMap)

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)

	schemaURL := "https://example.com/schema.json"
	if err := compiler.AddResource(schemaURL, schemaMap); err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}

	compiledSchema, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return &jsonSchemaValidator{
		compiler: compiler,
		schema:   compiledSchema,
	}, nil
}

// transformOpenAPIToJSONSchema converts OpenAPI 3.0/3.1 specific keywords to JSON Schema format
func transformOpenAPIToJSONSchema(schema map[string]any) {
	// Handle nullable - in OpenAPI 3.0, nullable is a boolean flag
	// In OpenAPI 3.1 / JSON Schema 2020-12, we use type arrays
	if nullable, ok := schema["nullable"].(bool); ok && nullable {
		if typeVal, ok := schema["type"].(string); ok {
			// Convert to type array with null (must be []any for jsonschema compiler)
			schema["type"] = []any{typeVal, "null"}
		}
		// If no type is set, don't add a type constraint - nullable just means null is
		// allowed alongside whatever values the schema permits (OpenAPI 3.0 semantics).
		delete(schema, "nullable")
	}

	// Handle exclusiveMinimum/exclusiveMaximum
	// In OpenAPI 3.0, these are booleans alongside minimum/maximum
	// In JSON Schema 2020-12, they are numeric values
	if exclusiveMin, ok := schema["exclusiveMinimum"].(bool); ok {
		if exclusiveMin {
			if schemaMin, ok := schema["minimum"].(float64); ok {
				schema["exclusiveMinimum"] = schemaMin
				delete(schema, "minimum")
			} else {
				delete(schema, "exclusiveMinimum")
			}
		} else {
			// exclusiveMinimum: false means inclusive, which is the JSON Schema default
			delete(schema, "exclusiveMinimum")
		}
	}
	if exclusiveMax, ok := schema["exclusiveMaximum"].(bool); ok {
		if exclusiveMax {
			if schemaMax, ok := schema["maximum"].(float64); ok {
				schema["exclusiveMaximum"] = schemaMax
				delete(schema, "maximum")
			} else {
				delete(schema, "exclusiveMaximum")
			}
		} else {
			// exclusiveMaximum: false means inclusive, which is the JSON Schema default
			delete(schema, "exclusiveMaximum")
		}
	}

	// Remove OpenAPI-specific keywords that aren't in JSON Schema
	delete(schema, "discriminator")
	delete(schema, "xml")
	delete(schema, "externalDocs")
	delete(schema, "example") // Use "examples" in 2020-12

	// Recursively transform nested schemas (single schema fields)
	for _, key := range []string{
		"additionalProperties", "items", "not",
		// OpenAPI 3.1 / JSON Schema 2020-12 fields
		"contains", "propertyNames", "unevaluatedItems", "unevaluatedProperties",
		"if", "then", "else", "contentSchema",
	} {
		if val, ok := schema[key]; ok {
			if nestedSchema, ok := val.(map[string]any); ok {
				transformOpenAPIToJSONSchema(nestedSchema)
			}
		}
	}

	// Transform schema arrays (oneOf, anyOf, allOf, prefixItems)
	for _, key := range []string{"oneOf", "anyOf", "allOf", "prefixItems"} {
		if val, ok := schema[key].([]any); ok {
			for _, item := range val {
				if nestedSchema, ok := item.(map[string]any); ok {
					transformOpenAPIToJSONSchema(nestedSchema)
				}
			}
		}
	}

	// Transform schema maps (properties, patternProperties, dependentSchemas, $defs)
	for _, key := range []string{"properties", "patternProperties", "dependentSchemas", "$defs"} {
		if props, ok := schema[key].(map[string]any); ok {
			for _, propVal := range props {
				if propSchema, ok := propVal.(map[string]any); ok {
					transformOpenAPIToJSONSchema(propSchema)
				}
			}
		}
	}
}

// validate validates a value against the compiled JSON Schema.
// The original schema and settings are passed so that SchemaError instances
// can be fully populated (Value, Schema, customizeMessageError).
func (v *jsonSchemaValidator) validate(value any, schema *Schema, settings *schemaValidationSettings) error {
	if err := v.schema.Validate(value); err != nil {
		// Convert jsonschema error to SchemaError
		return convertJSONSchemaError(err, value, schema, settings)
	}
	return nil
}

// convertJSONSchemaError converts a jsonschema validation error to OpenAPI SchemaError format
func convertJSONSchemaError(err error, value any, schema *Schema, settings *schemaValidationSettings) error {
	var validationErr *jsonschema.ValidationError
	if errors.As(err, &validationErr) {
		return formatValidationError(validationErr, "", value, schema, settings)
	}
	return err
}

// formatValidationError recursively formats validation errors into SchemaError instances.
// value and schema are attached to each leaf SchemaError so that SchemaError.Error()
// can print schema/value details and so that customizeMessageError (user-supplied via
// SetSchemaErrorMessageCustomizer) is called with a fully-populated SchemaError.
func formatValidationError(verr *jsonschema.ValidationError, parentPath string, value any, schema *Schema, settings *schemaValidationSettings) error {
	// Build the path from InstanceLocation slice
	path := "/" + strings.Join(verr.InstanceLocation, "/")
	if parentPath != "" && path != "/" {
		path = parentPath + path
	} else if path == "/" {
		path = parentPath
	}

	// Build error message using the Error() method
	var msg strings.Builder
	if path != "" {
		msg.WriteString(fmt.Sprintf(`error at "%s": `, path))
	}
	msg.WriteString(verr.Error())

	// If there are sub-errors, format them too
	if len(verr.Causes) > 0 {
		var subErrors MultiError
		for _, cause := range verr.Causes {
			if subErr := formatValidationError(cause, path, value, schema, settings); subErr != nil {
				subErrors = append(subErrors, subErr)
			}
		}
		if len(subErrors) > 0 {
			se := &SchemaError{
				Value:  value,
				Schema: schema,
				Reason: msg.String(),
				Origin: fmt.Errorf("validation failed due to: %w", subErrors),
			}
			if settings != nil {
				se.customizeMessageError = settings.customizeMessageError
			}
			return se
		}
	}

	se := &SchemaError{
		Value:  value,
		Schema: schema,
		Reason: msg.String(),
	}
	if settings != nil {
		se.customizeMessageError = settings.customizeMessageError
	}
	return se
}

// visitJSONWithJSONSchema validates using the JSON Schema 2020-12 validator.
// If schema compilation fails (e.g., because the schema contains $ref values
// that are relative to the parent document and cannot be resolved in the
// standalone compilation context), it falls back to the built-in validator.
func (schema *Schema) visitJSONWithJSONSchema(settings *schemaValidationSettings, value any) error {
	validator, err := newJSONSchemaValidator(schema)
	if err != nil {
		// Fall back to built-in validator when the JSON Schema compiler cannot
		// resolve document-relative $refs. This is expected for schemas that
		// reference other components via #/components/... paths.
		return schema.visitJSON(settings, value)
	}

	return validator.validate(value, schema, settings)
}
