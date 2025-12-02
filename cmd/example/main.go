package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/openai/openai-go/v3"
)

// OpenAPISpec represents an OpenAPI 3.x specification
type OpenAPISpec struct {
	OpenAPI string               `json:"openapi"`
	Paths   map[string]PathItem  `json:"paths"`
	Info    Info                 `json:"info"`
}

// Info contains API metadata
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// PathItem represents a path in the OpenAPI spec
type PathItem struct {
	Get    *Operation `json:"get"`
	Post   *Operation `json:"post"`
	Put    *Operation `json:"put"`
	Delete *Operation `json:"delete"`
	Patch  *Operation `json:"patch"`
}

// Operation represents an API operation
type Operation struct {
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Parameters  []Parameter `json:"parameters,omitempty"`
	RequestBody *RequestBody `json:"requestBody,omitempty"`
}

// Parameter represents an API parameter
type Parameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Schema      *Schema     `json:"schema"`
}

// RequestBody represents the request body definition
type RequestBody struct {
	Content map[string]MediaType `json:"content"`
}

// MediaType represents media type in request/response body
type MediaType struct {
	Schema *Schema `json:"schema"`
}

// Schema represents JSON Schema
type Schema struct {
	Type       string             `json:"type"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Required   []string           `json:"required,omitempty"`
	Format     string             `json:"format,omitempty"`
	Description string           `json:"description,omitempty"`
}

// FunctionDefinition represents an LLM function definition
type FunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// APIClient handles HTTP requests to the API
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
}

// ConvertOpenAPIToFunctions converts OpenAPI spec to LLM function definitions
func ConvertOpenAPIToFunctions(spec *OpenAPISpec, baseURL string) []FunctionDefinition {
	var functions []FunctionDefinition
	client := NewAPIClient(baseURL)

	for path, pathItem := range spec.Paths {
		for method, op := range pathItem.getOperations() {
			if op == nil {
				continue
			}
			
			funcDef := FunctionDefinition{
				Name:        sanitizeFunctionName(path, method),
				Description: op.Summary,
			}
			
			// Build parameters schema
			params := map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			}
			
			// Handle path and query parameters
			for _, param := range op.Parameters {
				if param.In == "path" || param.In == "query" {
					prop := convertSchemaToProperty(param.Schema)
					prop["description"] = param.Description
					params["properties"].(map[string]interface{})[param.Name] = prop
					
					if param.Required {
						params["required"] = append(params["required"].([]string), param.Name)
					}
				}
			}
			
			// Handle request body
			if op.RequestBody != nil {
				for _, mediaType := range op.RequestBody.Content {
					if mediaType.Schema != nil {
						reqBodySchema := convertSchemaToProperty(mediaType.Schema)
						params["properties"].(map[string]interface{})["requestBody"] = reqBodySchema
					}
				}
			}
			
			funcDef.Parameters = params
			functions = append(functions, funcDef)
			
			// Register handler
			registerFunctionHandler(funcDef.Name, method, path, client)
		}
	}
	
	return functions
}

// Helper to get operations from path item
func (p *PathItem) getOperations() map[string]*Operation {
	ops := make(map[string]*Operation)
	if p.Get != nil {
		ops["GET"] = p.Get
	}
	if p.Post != nil {
		ops["POST"] = p.Post
	}
	if p.Put != nil {
		ops["PUT"] = p.Put
	}
	if p.Delete != nil {
		ops["DELETE"] = p.Delete
	}
	if p.Patch != nil {
		ops["PATCH"] = p.Patch
	}
	return ops
}

// Convert OpenAPI schema to function parameter property
func convertSchemaToProperty(schema *Schema) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{"type": "string"}
	}
	
	prop := map[string]interface{}{
		"type": schema.Type,
	}
	
	if schema.Description != "" {
		prop["description"] = schema.Description
	}
	
	if schema.Properties != nil {
		props := map[string]interface{}{}
		for name, subSchema := range schema.Properties {
			props[name] = convertSchemaToProperty(subSchema)
		}
		prop["properties"] = props
		
		if len(schema.Required) > 0 {
			prop["required"] = schema.Required
		}
	}
	
	return prop
}

// Sanitize function name
func sanitizeFunctionName(path, method string) string {
	name := strings.ReplaceAll(path, "/", "_")
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")
	name = method + name
	return strings.ToLower(name)
}

// Function handler registry
var functionHandlers = make(map[string]func(map[string]interface{}) (interface{}, error))

// Register function handler
func registerFunctionHandler(name, method, path string, client *APIClient) {
	functionHandlers[name] = func(args map[string]interface{}) (interface{}, error) {
		return executeAPIRequest(client, method, path, args)
	}
}

// Execute API request
func executeAPIRequest(client *APIClient, method, path string, args map[string]interface{}) (interface{}, error) {
	// Build URL with path parameters
	url := client.BaseURL + path
	
	for key, value := range args {
		if strings.Contains(url, "{"+key+"}") {
			url = strings.ReplaceAll(url, "{"+key+"}", fmt.Sprintf("%v", value))
		}
	}
	
	var body []byte
	var err error
	
	if requestBody, ok := args["requestBody"]; ok {
		body, err = json.Marshal(requestBody)
		if err != nil {
			return nil, err
		}
	}
	
	req, err := http.NewRequest(method, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result, nil
}

// ExecuteFunction calls the registered function handler
func ExecuteFunction(name string, arguments map[string]interface{}) (interface{}, error) {
	handler, ok := functionHandlers[name]
	if !ok {
		return nil, fmt.Errorf("function %s not found", name)
	}
	
	return handler(arguments)
}

// Example usage
func main() {
	// Example OpenAPI spec
	specJSON := `{
    "openapi": "3.0.0",
    "info": {
      "title": "Pet Store API",
      "version": "1.0.0"
    },
    "paths": {
      "/pets": {
        "get": {
          "summary": "List all pets",
          "parameters": [
            {
              "name": "limit",
              "in": "query",
              "description": "Maximum number of pets to return",
              "required": false,
              "schema": {
                "type": "integer",
                "format": "int32"
              }
            }
          ]
        },
        "post": {
          "summary": "Create a pet",
          "requestBody": {
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "name": {
                      "type": "string",
                      "description": "Pet name"
                    },
                    "tag": {
                      "type": "string",
                      "description": "Pet tag"
                    }
                  },
                  "required": ["name"]
                }
              }
            }
          }
        }
      },
      "/pets/{id}": {
        "get": {
          "summary": "Get a pet by ID",
          "parameters": [
            {
              "name": "id",
              "in": "path",
              "description": "Pet identifier",
              "required": true,
              "schema": {
                "type": "integer",
                "format": "int64"
              }
            }
          ]
        }
      }
    }
  }`
	
	var spec OpenAPISpec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		log.Fatal(err)
	}
	
	// Convert to functions
	functions := ConvertOpenAPIToFunctions(&spec, "https://petstore.swagger.io/v2")
	
	// Print generated functions
	for _, fn := range functions {
		fmt.Printf("Function: %s\n", fn.Name)
		fmt.Printf("Description: %s\n", fn.Description)
		fmt.Printf("Parameters: %+v\n\n", fn.Parameters)
	}
	
	// Example of executing a function
	// In a real implementation, you would call this after getting function name and arguments from LLM
	// result, err := ExecuteFunction("get_pets", map[string]interface{}{"limit": 10})
	// if err != nil {
	//     log.Printf("Error executing function: %v", err)
	// } else {
	//     fmt.Printf("Function result: %+v\n", result)
	// }
	
	// Example using OpenAI SDK
	ctx := context.Background()
	client := openai.NewClient()
	
	// Prepare functions for OpenAI
	var openAIFuncs []openai.ChatCompletionFunction
	for _, fn := range functions {
		jsonBytes, _ := json.Marshal(fn.Parameters)
		openAIFuncs = append(openAIFuncs, openai.ChatCompletionFunction{
			Name:        fn.Name,
			Description: &fn.Description,
			Parameters:  jsonBytes,
		})
	}
	
	// Make request to OpenAI
	resp, err := client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model: openai.F(openai.ChatModelGPT4Turbo),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.NewUserMessage("What pets are available with limit 5?"),
			},
			Functions: openAIFuncs,
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	
	// Handle function call response
	for _, choice := range resp.Choices {
		if choice.Message.FunctionCall != nil {
			fmt.Printf("Function call: %s\n", *choice.Message.FunctionCall.Name)
			fmt.Printf("Arguments: %s\n", *choice.Message.FunctionCall.Arguments)
			
			// Parse arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(*choice.Message.FunctionCall.Arguments), &args); err != nil {
				log.Printf("Error parsing arguments: %v", err)
				continue
			}
			
			// Execute the function
			result, err := ExecuteFunction(*choice.Message.FunctionCall.Name, args)
			if err != nil {
				log.Printf("Error executing function: %v", err)
			} else {
				fmt.Printf("Function result: %+v\n", result)
			}
		}
	}
}