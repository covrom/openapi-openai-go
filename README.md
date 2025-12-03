# openapi-openai-go
Convert OpenAPI specifications to OpenAI function calling schemas and execute them with the official OpenAI Go SDK.

## Features
- Parse OpenAPI 3.0 specifications from files or URLs
- Convert OpenAPI operations to OpenAI function definitions
- Automatic schema transformation (parameters, request bodies, responses)
- Execute function calls using the official openai-go/v3 SDK
- Support for complex data types, enums, and validation rules
- Authentication support (Basic, Bearer, API Key, OAuth2, Cookie)
- Integration with OpenAI's function calling tools API
- Complete example demonstrating usage with OpenAI models

## Installation
```bash
go get github.com/covrom/openapi-openai-go
```

## Usage
```go
// Parse OpenAPI spec
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
        }
      }
    }
  }`

var spec apiai.OpenAPISpec
json.Unmarshal([]byte(specJSON), &spec)

// Convert to OpenAI functions
functions := apiai.ConvertOpenAPIToFunctions(&spec)

// Create API client
client, err := apiai.NewAPIClient("https://petstore.swagger.io/v2", nil)
if err != nil {
    log.Fatal(err)
}

// Execute function
result, err := apiai.ExecuteFunction(client, functions["get_pets"], map[string]interface{}{"limit": 10})
if err != nil {
    log.Fatal(err)
}
println(result)
```

## Examples
See the `cmd/example` directory for a complete working example that demonstrates integration with OpenAI's function calling API.
