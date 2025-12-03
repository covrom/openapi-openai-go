package apiai

import (
	"encoding/json"
	"testing"
)

func TestResolveParameterRef(t *testing.T) {
	// Create a test spec with components parameters
	spec := &OpenAPISpec{
		Components: &Components{
			Parameters: map[string]Parameter{
				"offsetParam": {
					Name:        "offset",
					In:          "query",
					Description: "The number of items to skip before starting to collect the result set.",
					Required:    false,
					Schema: &Schema{
						Type: "integer",
					},
				},
				"limitParam": {
					Name:        "limit",
					In:          "query",
					Description: "The numbers of items to return.",
					Required:    false,
					Schema: &Schema{
						Type: "integer",
					},
				},
			},
		},
	}

	// Test resolving a valid $ref
	paramWithRef := Parameter{
		Ref: "#/components/parameters/offsetParam",
	}

	resolved := resolveParameterRef(paramWithRef, spec)
	if resolved.Name != "offset" {
		t.Errorf("Expected parameter name 'offset', got '%s'", resolved.Name)
	}
	if resolved.In != "query" {
		t.Errorf("Expected parameter in 'query', got '%s'", resolved.In)
	}
	if resolved.Description != "The number of items to skip before starting to collect the result set." {
		t.Errorf("Unexpected description: %s", resolved.Description)
	}

	// Test resolving a non-existent $ref
	paramWithInvalidRef := Parameter{
		Ref: "#/components/parameters/nonExistentParam",
	}

	resolvedInvalid := resolveParameterRef(paramWithInvalidRef, spec)
	// Should return the original parameter when ref is not found
	if resolvedInvalid.Ref != "#/components/parameters/nonExistentParam" {
		t.Errorf("Expected original parameter to be returned for invalid ref")
	}

	// Test parameter without $ref
	paramWithoutRef := Parameter{
		Name: "directParam",
		In:   "query",
	}

	resolvedDirect := resolveParameterRef(paramWithoutRef, spec)
	if resolvedDirect.Name != "directParam" {
		t.Errorf("Expected parameter name 'directParam', got '%s'", resolvedDirect.Name)
	}
}

func TestConvertOpenAPIToFunctionsWithRefs(t *testing.T) {
	// Create a test OpenAPI spec with $ref parameters
	specJSON := `{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"components": {
			"parameters": {
				"offsetParam": {
					"in": "query",
					"name": "offset",
					"required": false,
					"schema": {
						"type": "integer",
						"minimum": 0
					},
					"description": "The number of items to skip before starting to collect the result set."
				},
				"limitParam": {
					"in": "query",
					"name": "limit",
					"required": false,
					"schema": {
						"type": "integer",
						"minimum": 1,
						"maximum": 50,
						"default": 20
					},
					"description": "The numbers of items to return."
				}
			}
		},
		"paths": {
			"/users": {
				"get": {
					"summary": "Gets a list of users.",
					"parameters": [
						{"$ref": "#/components/parameters/offsetParam"},
						{"$ref": "#/components/parameters/limitParam"}
					],
					"responses": {
						"200": {
							"description": "OK"
						}
					}
				}
			},
			"/teams": {
				"get": {
					"summary": "Gets a list of teams.",
					"parameters": [
						{"$ref": "#/components/parameters/offsetParam"},
						{"$ref": "#/components/parameters/limitParam"}
					],
					"responses": {
						"200": {
							"description": "OK"
						}
					}
				}
			}
		}
	}`

	var spec OpenAPISpec
	err := json.Unmarshal([]byte(specJSON), &spec)
	if err != nil {
		t.Fatalf("Failed to unmarshal test spec: %v", err)
	}

	functions := ConvertOpenAPIToFunctions(&spec)

	// Check that we have the expected functions
	if len(functions) != 2 {
		t.Errorf("Expected 2 functions, got %d", len(functions))
	}

	// Check the users function
	usersFunc, exists := functions["get_users"]
	if !exists {
		t.Errorf("Expected get_users function to exist")
	}

	if usersFunc.Name != "get_users" {
		t.Errorf("Expected function name 'get_users', got '%s'", usersFunc.Name)
	}

	// Check that the parameters were resolved correctly
	if len(usersFunc.Parameters.Properties) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(usersFunc.Parameters.Properties))
	}

	// Check offset parameter
	offsetParam, exists := usersFunc.Parameters.Properties["offset"]
	if !exists {
		t.Errorf("Expected offset parameter to exist")
	}
	if offsetParam.Type != "integer" {
		t.Errorf("Expected offset parameter type 'integer', got '%s'", offsetParam.Type)
	}

	// Check limit parameter
	limitParam, exists := usersFunc.Parameters.Properties["limit"]
	if !exists {
		t.Errorf("Expected limit parameter to exist")
	}
	if limitParam.Type != "integer" {
		t.Errorf("Expected limit parameter type 'integer', got '%s'", limitParam.Type)
	}

	// Check query params
	if len(usersFunc.QueryParams) != 2 {
		t.Errorf("Expected 2 query params, got %d", len(usersFunc.QueryParams))
	}

	// Check the teams function
	teamsFunc, exists := functions["get_teams"]
	if !exists {
		t.Errorf("Expected get_teams function to exist")
	}

	if teamsFunc.Name != "get_teams" {
		t.Errorf("Expected function name 'get_teams', got '%s'", teamsFunc.Name)
	}

	// Teams should have the same parameters
	if len(teamsFunc.Parameters.Properties) != 2 {
		t.Errorf("Expected 2 parameters for teams, got %d", len(teamsFunc.Parameters.Properties))
	}
}

func TestUnmarshalOpenAPISpecFromYAML(t *testing.T) {
	// Test YAML data
	specYAML := `
openapi: 3.0.0
info:
  title: Test API
  description: A test API
  version: 1.0.0
paths:
  /test:
    get:
      summary: Get test data
      description: Returns test data
      parameters:
        - name: id
          in: query
          description: Test ID
          required: true
          schema:
            type: string
`

	// Test YAML unmarshaling
	spec, err := UnmarshalOpenAPISpecFromYAML([]byte(specYAML))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify the unmarshaled data
	if spec.Info.Title != "Test API" {
		t.Errorf("Expected title 'Test API', got '%s'", spec.Info.Title)
	}

	if spec.Info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", spec.Info.Version)
	}

	if len(spec.Paths) != 1 {
		t.Errorf("Expected 1 path, got %d", len(spec.Paths))
	}

	testPath, exists := spec.Paths["/test"]
	if !exists {
		t.Errorf("Expected /test path to exist")
	}

	if testPath.Get == nil {
		t.Errorf("Expected GET operation to exist")
	}

	if testPath.Get.Summary != "Get test data" {
		t.Errorf("Expected summary 'Get test data', got '%s'", testPath.Get.Summary)
	}

	// Test conversion to functions
	functions := ConvertOpenAPIToFunctions(spec)
	if len(functions) != 1 {
		t.Errorf("Expected 1 function, got %d", len(functions))
	}

	testFunc, exists := functions["get_test"]
	if !exists {
		t.Errorf("Expected get_test function to exist")
	}

	if testFunc.Description != "Get test data\nReturns test data" {
		t.Errorf("Expected description 'Get test data\nReturns test data', got '%s'", testFunc.Description)
	}
}

func TestUnmarshalOpenAPISpecAutoDetect(t *testing.T) {
	// Test JSON data
	specJSON := `{
		"openapi": "3.0.0",
		"info": {
			"title": "JSON Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/json": {
				"get": {
					"summary": "JSON test endpoint"
				}
			}
		}
	}`

	// Test JSON auto-detection
	spec, err := UnmarshalOpenAPISpec([]byte(specJSON))
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON with auto-detect: %v", err)
	}

	if spec.Info.Title != "JSON Test API" {
		t.Errorf("Expected title 'JSON Test API', got '%s'", spec.Info.Title)
	}

	// Test YAML data
	specYAML := `
openapi: 3.0.0
info:
  title: YAML Test API
  version: 1.0.0
paths:
  /yaml:
    get:
      summary: YAML test endpoint
`

	// Test YAML auto-detection
	spec, err = UnmarshalOpenAPISpec([]byte(specYAML))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML with auto-detect: %v", err)
	}

	if spec.Info.Title != "YAML Test API" {
		t.Errorf("Expected title 'YAML Test API', got '%s'", spec.Info.Title)
	}
}
