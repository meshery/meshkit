package component

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestGetResolvedManifest(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
		// checkPath is a dot-separated path into the parsed output to verify
		// that $ref was resolved. Empty means just check valid JSON output.
		checkPath string
		wantType  string
	}{
		{
			name: "No refs passes through unchanged",
			input: `{
				"openapi": "3.0.0",
				"info": {"title": "test", "version": "1.0"},
				"paths": {},
				"components": {
					"schemas": {
						"Foo": {
							"type": "object",
							"properties": {
								"name": {"type": "string"}
							}
						}
					}
				}
			}`,
			checkPath: "components.schemas.Foo.properties.name",
			wantType:  "string",
		},
		{
			name: "Property $ref is inlined",
			input: `{
				"openapi": "3.0.0",
				"info": {"title": "test", "version": "1.0"},
				"paths": {},
				"components": {
					"schemas": {
						"Address": {
							"type": "object",
							"properties": {
								"street": {"type": "string"}
							}
						},
						"Person": {
							"type": "object",
							"properties": {
								"name": {"type": "string"},
								"address": {"$ref": "#/components/schemas/Address"}
							}
						}
					}
				}
			}`,
			checkPath: "components.schemas.Person.properties.address",
			wantType:  "object",
		},
		{
			name: "Nested $ref chain is fully inlined",
			input: `{
				"openapi": "3.0.0",
				"info": {"title": "test", "version": "1.0"},
				"paths": {},
				"components": {
					"schemas": {
						"Zip": {"type": "string"},
						"Address": {
							"type": "object",
							"properties": {
								"zip": {"$ref": "#/components/schemas/Zip"}
							}
						},
						"Person": {
							"type": "object",
							"properties": {
								"address": {"$ref": "#/components/schemas/Address"}
							}
						}
					}
				}
			}`,
			checkPath: "components.schemas.Person.properties.address.properties.zip",
			wantType:  "string",
		},
		{
			name: "Array items $ref is inlined",
			input: `{
				"openapi": "3.0.0",
				"info": {"title": "test", "version": "1.0"},
				"paths": {},
				"components": {
					"schemas": {
						"Tag": {
							"type": "object",
							"properties": {"label": {"type": "string"}}
						},
						"TagList": {
							"type": "array",
							"items": {"$ref": "#/components/schemas/Tag"}
						}
					}
				}
			}`,
			checkPath: "components.schemas.TagList.items",
			wantType:  "object",
		},
		{
			name: "additionalProperties $ref is inlined",
			input: `{
				"openapi": "3.0.0",
				"info": {"title": "test", "version": "1.0"},
				"paths": {},
				"components": {
					"schemas": {
						"Value": {"type": "string"},
						"Map": {
							"type": "object",
							"additionalProperties": {"$ref": "#/components/schemas/Value"}
						}
					}
				}
			}`,
			checkPath: "components.schemas.Map.additionalProperties",
			wantType:  "string",
		},
		{
			name: "YAML input is accepted",
			input: `
openapi: "3.0.0"
info:
  title: test
  version: "1.0"
paths: {}
components:
  schemas:
    Foo:
      type: object
      properties:
        name:
          type: string
`,
			checkPath: "components.schemas.Foo.properties.name",
			wantType:  "string",
		},
		{
			name: "No components returns ErrNoSchemasFound",
			input: `{
				"openapi": "3.0.0",
				"info": {"title": "test", "version": "1.0"},
				"paths": {}
			}`,
			wantErr:   true,
			errSubstr: "schema",
		},
		{
			name:      "Invalid input returns error",
			input:     "{{invalid",
			wantErr:   true,
			errSubstr: "",
		},
		{
			name: "Self-referencing schema like JSONSchemaProps does not stack overflow",
			input: `{
				"openapi": "3.0.0",
				"info": {"title": "k8s-like", "version": "1.0"},
				"paths": {},
				"components": {
					"schemas": {
						"JSONSchemaProps": {
							"type": "object",
							"properties": {
								"description": {"type": "string"},
								"properties": {
									"type": "object",
									"additionalProperties": {"$ref": "#/components/schemas/JSONSchemaProps"}
								},
								"items": {
									"$ref": "#/components/schemas/JSONSchemaPropsOrArray"
								},
								"not": {
									"$ref": "#/components/schemas/JSONSchemaProps"
								},
								"allOf": {
									"type": "array",
									"items": {"$ref": "#/components/schemas/JSONSchemaProps"}
								}
							}
						},
						"JSONSchemaPropsOrArray": {
							"type": "object",
							"properties": {
								"schema": {"$ref": "#/components/schemas/JSONSchemaProps"},
								"jsonSchemas": {
									"type": "array",
									"items": {"$ref": "#/components/schemas/JSONSchemaProps"}
								}
							}
						}
					}
				}
			}`,
			checkPath: "components.schemas.JSONSchemaProps.properties.description",
			wantType:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := getResolvedManifest(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var parsed map[string]any
			if err := json.Unmarshal([]byte(out), &parsed); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}

			if tt.checkPath == "" {
				return
			}

			node := navigatePath(t, parsed, tt.checkPath)
			m, ok := node.(map[string]any)
			if !ok {
				t.Fatalf("expected map at path %s, got %T", tt.checkPath, node)
			}
			if _, hasRef := m["$ref"]; hasRef {
				t.Errorf("$ref still present at path %s", tt.checkPath)
			}
			if tt.wantType != "" {
				if got := m["type"]; got != tt.wantType {
					t.Errorf("type at %s = %v, want %s", tt.checkPath, got, tt.wantType)
				}
			}
		})
	}
}

func TestGetResolvedManifest_AllOf(t *testing.T) {
	input := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1.0"},
		"paths": {},
		"components": {
			"schemas": {
				"Base": {
					"type": "object",
					"properties": {"id": {"type": "integer"}}
				},
				"Extended": {
					"allOf": [
						{"$ref": "#/components/schemas/Base"},
						{"type": "object", "properties": {"name": {"type": "string"}}}
					]
				}
			}
		}
	}`

	out, err := getResolvedManifest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(out), &parsed)

	extended := navigatePath(t, parsed, "components.schemas.Extended").(map[string]any)
	allOf, ok := extended["allOf"].([]any)
	if !ok {
		t.Fatal("expected allOf to be an array")
	}

	for i, item := range allOf {
		m := item.(map[string]any)
		if _, hasRef := m["$ref"]; hasRef {
			t.Errorf("allOf[%d] still has $ref", i)
		}
	}

	base := allOf[0].(map[string]any)
	if base["type"] != "object" {
		t.Errorf("allOf[0] type = %v, want object", base["type"])
	}
}

func TestClearSchemaRefs(t *testing.T) {
	tests := []struct {
		name string
		sr   *openapi3.SchemaRef
	}{
		{
			name: "Nil SchemaRef does not panic",
			sr:   nil,
		},
		{
			name: "Nil Value clears Ref",
			sr:   &openapi3.SchemaRef{Ref: "#/components/schemas/Foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visited := make(map[*openapi3.Schema]bool)
			clearSchemaRefs(tt.sr, visited)
			if tt.sr != nil && tt.sr.Ref != "" {
				t.Errorf("Ref = %q, want empty", tt.sr.Ref)
			}
		})
	}
}

func TestClearSchemaRefs_Circular(t *testing.T) {
	// Build a circular reference: A -> B -> A
	a := &openapi3.SchemaRef{Ref: "#/components/schemas/A", Value: &openapi3.Schema{}}
	b := &openapi3.SchemaRef{Ref: "#/components/schemas/B", Value: &openapi3.Schema{}}
	a.Value.Properties = openapi3.Schemas{"b": b}
	b.Value.Properties = openapi3.Schemas{"a": a}

	stack := make(map[*openapi3.Schema]bool)
	clearSchemaRefs(a, stack) // must not hang or panic

	if a.Ref != "" {
		t.Errorf("a.Ref = %q, want empty", a.Ref)
	}
	if b.Ref != "" {
		t.Errorf("b.Ref = %q, want empty", b.Ref)
	}

	// json.Marshal must not stack overflow on the cleared structure.
	if _, err := json.Marshal(a); err != nil {
		t.Fatalf("json.Marshal failed after clearing circular refs: %v", err)
	}
}

func TestClearSchemaRefs_SelfReference(t *testing.T) {
	// Schema that references itself (like JSONSchemaProps).
	self := &openapi3.SchemaRef{Value: &openapi3.Schema{
		Type: &openapi3.Types{"object"},
	}}
	self.Value.Properties = openapi3.Schemas{"nested": self}

	stack := make(map[*openapi3.Schema]bool)
	clearSchemaRefs(self, stack)

	// The self-referencing property should be replaced, breaking the cycle.
	if _, err := json.Marshal(self); err != nil {
		t.Fatalf("json.Marshal failed on self-referencing schema: %v", err)
	}
}

func TestGetResolvedManifest_PathsAndComponents(t *testing.T) {
	input := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1.0"},
		"paths": {
			"/pets": {
				"get": {
					"parameters": [
						{"$ref": "#/components/parameters/LimitParam"}
					],
					"responses": {
						"200": {
							"$ref": "#/components/responses/PetList"
						}
					}
				},
				"post": {
					"requestBody": {
						"$ref": "#/components/requestBodies/PetBody"
					},
					"responses": {
						"201": {
							"description": "created"
						}
					}
				}
			}
		},
		"components": {
			"schemas": {
				"Pet": {
					"type": "object",
					"properties": {
						"name": {"type": "string"}
					}
				}
			},
			"parameters": {
				"LimitParam": {
					"name": "limit",
					"in": "query",
					"schema": {"type": "integer"}
				}
			},
			"requestBodies": {
				"PetBody": {
					"content": {
						"application/json": {
							"schema": {"$ref": "#/components/schemas/Pet"}
						}
					}
				}
			},
			"responses": {
				"PetList": {
					"description": "A list of pets",
					"content": {
						"application/json": {
							"schema": {
								"type": "array",
								"items": {"$ref": "#/components/schemas/Pet"}
							}
						}
					}
				}
			}
		}
	}`

	out, err := getResolvedManifest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify parameter ref in path is resolved.
	param := navigatePath(t, parsed, "paths./pets.get.parameters")
	params, ok := param.([]any)
	if !ok || len(params) == 0 {
		t.Fatal("expected parameters array")
	}
	pm := params[0].(map[string]any)
	if _, hasRef := pm["$ref"]; hasRef {
		t.Error("parameter $ref still present in path")
	}
	if pm["name"] != "limit" {
		t.Errorf("parameter name = %v, want limit", pm["name"])
	}

	// Verify response ref in path is resolved.
	resp := navigatePath(t, parsed, "paths./pets.get.responses.200").(map[string]any)
	if _, hasRef := resp["$ref"]; hasRef {
		t.Error("response $ref still present in path")
	}
	if resp["description"] != "A list of pets" {
		t.Errorf("response description = %v, want 'A list of pets'", resp["description"])
	}

	// Verify schema ref inside response content is resolved.
	items := navigatePath(t, parsed, "paths./pets.get.responses.200.content.application/json.schema.items").(map[string]any)
	if _, hasRef := items["$ref"]; hasRef {
		t.Error("schema $ref in response items still present")
	}
	if items["type"] != "object" {
		t.Errorf("items type = %v, want object", items["type"])
	}

	// Verify requestBody ref in path is resolved.
	rb := navigatePath(t, parsed, "paths./pets.post.requestBody").(map[string]any)
	if _, hasRef := rb["$ref"]; hasRef {
		t.Error("requestBody $ref still present in path")
	}

	// Verify schema ref inside requestBody content is resolved.
	rbSchema := navigatePath(t, parsed, "paths./pets.post.requestBody.content.application/json.schema").(map[string]any)
	if _, hasRef := rbSchema["$ref"]; hasRef {
		t.Error("schema $ref in requestBody content still present")
	}
	if rbSchema["type"] != "object" {
		t.Errorf("requestBody schema type = %v, want object", rbSchema["type"])
	}
}

func TestGetResolvedManifest_HeaderRefs(t *testing.T) {
	input := `{
		"openapi": "3.0.0",
		"info": {"title": "test", "version": "1.0"},
		"paths": {
			"/items": {
				"get": {
					"responses": {
						"200": {
							"description": "OK",
							"headers": {
								"X-Rate-Limit": {
									"$ref": "#/components/headers/RateLimit"
								}
							}
						}
					}
				}
			}
		},
		"components": {
			"schemas": {
				"Placeholder": {"type": "string"}
			},
			"headers": {
				"RateLimit": {
					"schema": {"type": "integer"}
				}
			}
		}
	}`

	out, err := getResolvedManifest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	header := navigatePath(t, parsed, "paths./items.get.responses.200.headers.X-Rate-Limit").(map[string]any)
	if _, hasRef := header["$ref"]; hasRef {
		t.Error("header $ref still present")
	}
	schema := header["schema"].(map[string]any)
	if schema["type"] != "integer" {
		t.Errorf("header schema type = %v, want integer", schema["type"])
	}
}

// navigatePath walks a dot-separated path through nested maps.
func navigatePath(t *testing.T, data map[string]any, path string) any {
	t.Helper()
	keys := splitDot(path)
	var current any = data
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected map at key %q in path %s, got %T", key, path, current)
		}
		current, ok = m[key]
		if !ok {
			t.Fatalf("key %q not found in path %s", key, path)
		}
	}
	return current
}

func splitDot(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
