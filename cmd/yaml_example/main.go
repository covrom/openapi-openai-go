package main

import (
	"fmt"
	"log"
	"github.com/covrom/openapi-openai-go"
)

func main() {
	// Example YAML OpenAPI spec
	yamlSpec := `
openapi: 3.0.0
info:
  title: Pet Store API
  description: A simple pet store API
  version: 1.0.0
paths:
  /pets:
    get:
      summary: List all pets
      description: Returns a list of all pets in the store
      parameters:
        - name: limit
          in: query
          description: How many items to return at one time (max 100)
          required: false
          schema:
            type: integer
            maximum: 100
            format: int32
      responses:
        '200':
          description: A paged array of pets
    post:
      summary: Create a pet
      description: Create a new pet in the store
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                status:
                  type: string
                  enum: [available, pending, sold]
              required:
                - name
      responses:
        '201':
          description: Pet created successfully
  /pets/{petId}:
    get:
      summary: Info for a specific pet
      description: Get detailed information about a specific pet
      parameters:
        - name: petId
          in: path
          required: true
          description: The id of the pet to retrieve
          schema:
            type: string
      responses:
        '200':
          description: Expected response to a valid request
`

	// Unmarshal from YAML
	spec, err := apiai.UnmarshalOpenAPISpecFromYAML([]byte(yamlSpec))
	if err != nil {
		log.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	fmt.Printf("Successfully loaded OpenAPI spec from YAML!\n")
	fmt.Printf("API Title: %s\n", spec.Info.Title)
	fmt.Printf("API Version: %s\n", spec.Info.Version)
	fmt.Printf("Number of paths: %d\n", len(spec.Paths))

	// Convert to function definitions
	functions := apiai.ConvertOpenAPIToFunctions(spec)
	fmt.Printf("Number of functions: %d\n\n", len(functions))

	// Display function information
	for name, fn := range functions {
		fmt.Printf("Function: %s\n", name)
		fmt.Printf("  Description: %s\n", fn.Description)
		fmt.Printf("  Method: %s\n", fn.OapiMethod)
		fmt.Printf("  Path: %s\n", fn.OapiPath)
		
		if len(fn.PathParams) > 0 {
			fmt.Printf("  Path Parameters: %v\n", fn.PathParams)
		}
		
		if len(fn.QueryParams) > 0 {
			fmt.Printf("  Query Parameters: %v\n", fn.QueryParams)
		}
		
		if len(fn.Parameters.Properties) > 0 {
			fmt.Printf("  Parameters:\n")
			for paramName, param := range fn.Parameters.Properties {
				required := false
				for _, req := range fn.Parameters.Required {
					if req == paramName {
						required = true
						break
					}
				}
				reqStr := "optional"
				if required {
					reqStr = "required"
				}
				fmt.Printf("    - %s (%s, %s): %s\n", paramName, param.Type, reqStr, param.Description)
			}
		}
		fmt.Println()
	}

	// Demonstrate auto-detection
	fmt.Println("=== Testing Auto-Detection ===")
	
	// Test with the same YAML data using auto-detection
	autoSpec, err := apiai.UnmarshalOpenAPISpec([]byte(yamlSpec))
	if err != nil {
		log.Fatalf("Failed to auto-detect format: %v", err)
	}
	
	fmt.Printf("Auto-detected YAML format successfully!\n")
	fmt.Printf("API Title: %s\n", autoSpec.Info.Title)
	
	// Test with JSON
	jsonSpec := `{
		"openapi": "3.0.0",
		"info": {
			"title": "JSON Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/test": {
				"get": {
					"summary": "Test endpoint"
				}
			}
		}
	}`
	
	autoJSONSpec, err := apiai.UnmarshalOpenAPISpec([]byte(jsonSpec))
	if err != nil {
		log.Fatalf("Failed to auto-detect JSON format: %v", err)
	}
	
	fmt.Printf("Auto-detected JSON format successfully!\n")
	fmt.Printf("API Title: %s\n", autoJSONSpec.Info.Title)
}
