# openapi-openai-go

A Go library that converts OpenAPI 3.0 specifications to OpenAI function calling schemas and executes them with the official OpenAI Go SDK. This library enables seamless integration between REST APIs described in OpenAPI format and OpenAI's function calling capabilities.

## Features

- **OpenAPI 3.0 Support**: Parse OpenAPI 3.0 specifications from JSON or YAML files/URLs
- **Function Schema Generation**: Convert OpenAPI operations to OpenAI function definitions automatically
- **Schema Transformation**: Automatic conversion of complex data types, enums, and validation rules
- **Multiple Authentication Methods**: Support for Basic, Bearer, API Key, OAuth2, and Cookie authentication
- **HTTP Client Integration**: Execute function calls using configurable HTTP clients with authentication
- **OpenAI SDK Integration**: Seamless integration with OpenAI's function calling tools API
- **Reference Resolution**: Handle `$ref` parameters in OpenAPI components
- **Auto-format Detection**: Automatically detect and parse JSON or YAML OpenAPI specs
- **Comprehensive Examples**: Complete examples demonstrating various use cases

## Installation

```bash
go get github.com/covrom/openapi-openai-go
```

## Quick Start

### Basic Usage

```go
package main

import (
    "encoding/json"
    "log"
    
    apiai "github.com/covrom/openapi-openai-go"
)

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
                }
            }
        }
    }`

    var spec apiai.OpenAPISpec
    if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
        log.Fatal(err)
    }

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
}
```

### Using with OpenAI SDK

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    
    apiai "github.com/covrom/openapi-openai-go"
    "github.com/openai/openai-go/v3"
)

func main() {
    // Load and convert OpenAPI spec
    spec := loadOpenAPISpec() // Your OpenAPI spec loading logic
    functions := apiai.ConvertOpenAPIToFunctions(spec)

    // Create API client
    apiClient, err := apiai.NewAPIClient("https://api.example.com", nil)
    if err != nil {
        log.Fatal(err)
    }

    // Setup OpenAI client
    openaiClient := openai.NewClient()
    ctx := context.Background()

    // Prepare tools for OpenAI
    var tools []openai.ChatCompletionToolUnionParam
    for _, fn := range functions {
        tools = append(tools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
            Name:        fn.Name,
            Description: openai.String(fn.Description),
            Parameters: openai.FunctionParameters{
                "type":       fn.Parameters.Type,
                "properties": fn.Parameters.Properties,
                "required":   fn.Parameters.Required,
            },
        }))
    }

    // Make request to OpenAI
    completion, err := openaiClient.Chat.Completions.New(
        ctx,
        openai.ChatCompletionNewParams{
            Model: openai.ChatModelGPT4o,
            Messages: []openai.ChatCompletionMessageParamUnion{
                openai.UserMessage("List all pets with limit 5"),
            },
            Tools: tools,
        },
    )
    if err != nil {
        log.Fatal(err)
    }

    // Handle tool calls
    toolCalls := completion.Choices[0].Message.ToolCalls
    if len(toolCalls) > 0 {
        for _, toolCall := range toolCalls {
            var args map[string]interface{}
            json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
            
            fn := functions[toolCall.Function.Name]
            result, err := apiai.ExecuteFunction(apiClient, fn, args)
            if err != nil {
                log.Printf("Error executing function: %v", err)
                continue
            }
            
            // Send result back to OpenAI for final response
            // ... handle tool message and get final completion
        }
    }
}
```

## Authentication

The library supports multiple authentication methods:

### Basic Authentication

```go
authConfig := &apiai.AuthConfig{
    Type:     apiai.AuthTypeBasic,
    Username: "admin",
    Password: "secret",
}

client, err := apiai.NewAPIClient("https://api.example.com", authConfig)
```

### API Key (Header)

```go
authConfig := &apiai.AuthConfig{
    Type:        apiai.AuthTypeAPIKeyHeader,
    APIKeyName:  "X-API-Key",
    APIKeyValue: "your-api-key",
}

client, err := apiai.NewAPIClient("https://api.example.com", authConfig)
```

### Bearer Token

```go
authConfig := &apiai.AuthConfig{
    Type:  apiai.AuthTypeBearer,
    Token: "your-bearer-token",
}

client, err := apiai.NewAPIClient("https://api.example.com", authConfig)
```

### OAuth2

```go
import "golang.org/x/oauth2"

oauth2Config := &oauth2.Config{
    // Your OAuth2 configuration
}
tokenSource := oauth2Config.TokenSource(context.Background(), token)

authConfig := &apiai.AuthConfig{
    Type:        apiai.AuthTypeOAuth2,
    TokenSource: tokenSource,
}

client, err := apiai.NewAPIClient("https://api.example.com", authConfig)
```

### Cookie Authentication

```go
authConfig := &apiai.AuthConfig{
    Type: apiai.AuthTypeCookie,
    Cookies: []*http.Cookie{
        {Name: "sessionid", Value: "abc123"},
        {Name: "csrftoken", Value: "xyz789"},
    },
}

client, err := apiai.NewAPIClient("https://api.example.com", authConfig)
```

## Working with OpenAPI Specifications

### Loading from JSON

```go
specJSON := []byte(`{"openapi": "3.0.0", ...}`)
spec, err := apiai.UnmarshalOpenAPISpecFromJSON(specJSON)
if err != nil {
    log.Fatal(err)
}
```

### Loading from YAML

```go
specYAML := []byte(`
openapi: 3.0.0
info:
  title: My API
  version: 1.0.0
paths:
  /users:
    get:
      summary: List users
`)

spec, err := apiai.UnmarshalOpenAPISpecFromYAML(specYAML)
if err != nil {
    log.Fatal(err)
}
```

### Auto-detection (JSON or YAML)

```go
specData := []byte(...) // Your OpenAPI spec data
spec, err := apiai.UnmarshalOpenAPISpec(specData)
if err != nil {
    log.Fatal(err)
}
```

### Handling References

The library automatically resolves `$ref` parameters in OpenAPI components:

```go
specJSON := `{
    "openapi": "3.0.0",
    "components": {
        "parameters": {
            "offsetParam": {
                "name": "offset",
                "in": "query",
                "schema": {"type": "integer"}
            }
        }
    },
    "paths": {
        "/users": {
            "get": {
                "parameters": [
                    {"$ref": "#/components/parameters/offsetParam"}
                ]
            }
        }
    }
}`

functions := apiai.ConvertOpenAPIToFunctions(spec)
// The offset parameter will be automatically resolved
```

## API Reference

### Core Types

#### `OpenAPISpec`
Represents an OpenAPI 3.0 specification with paths, components, and metadata.

#### `FunctionDefinition`
Represents a function definition compatible with OpenAI's function calling API:

```go
type FunctionDefinition struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Parameters  Schema   `json:"parameters"`
    OapiMethod  string   `json:"-"`      // Original HTTP method
    OapiPath    string   `json:"-"`      // Original OpenAPI path
    PathParams  []string `json:"-"`      // Path parameter names
    QueryParams []string `json:"-"`      // Query parameter names
}
```

#### `APIClient`
HTTP client for executing API calls with authentication:

```go
type APIClient struct {
    BaseURL    *url.URL
    HTTPClient *http.Client
}
```

### Main Functions

#### `ConvertOpenAPIToFunctions(spec *OpenAPISpec) map[string]*FunctionDefinition`
Converts OpenAPI operations to function definitions.

#### `ExecuteFunction(client *APIClient, fn *FunctionDefinition, arguments map[string]any) (any, error)`
Executes a function call against the target API.

#### `NewAPIClient(baseURL string, authConfig *AuthConfig, opts ...func(*http.Client)) (*APIClient, error)`
Creates a new API client with optional authentication.

#### `UnmarshalOpenAPISpec(data []byte) (*OpenAPISpec, error)`
Automatically detects and parses JSON or YAML OpenAPI specifications.

## Examples

The repository includes several comprehensive examples:

### 1. Basic Example (`cmd/example/main.go`)
Demonstrates basic usage with OpenAI integration and function calling.

### 2. Tool Call Example (`cmd/toolcall_example/tool_call_example.go`)
Shows how to handle OpenAI tool calls with mock function execution.

### 3. YAML Example (`cmd/yaml_example/main.go`)
Illustrates loading OpenAPI specs from YAML and auto-detection.

### Running Examples

```bash
# Basic example with OpenAI integration
go run cmd/example/main.go

# Tool calling example
go run cmd/toolcall_example/tool_call_example.go

# YAML parsing example
go run cmd/yaml_example/main.go
```

## Testing

Run the test suite:

```bash
go test ./...
```

The tests cover:
- OpenAPI spec parsing (JSON and YAML)
- Function conversion
- Parameter reference resolution
- Auto-detection of formats

## Requirements

- Go 1.25.4 or later
- OpenAI Go SDK v3.9.0+
- golang.org/x/oauth2 v0.33.0+
- gopkg.in/yaml.v3 v3.0.1+

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## Use Cases

This library is particularly useful for:

- **AI Agents**: Enable AI agents to interact with REST APIs through natural language
- **API Integration**: Quickly integrate existing REST APIs with OpenAI's function calling
- **Automation**: Build automated workflows that can interact with multiple APIs
- **Chatbots**: Create chatbots that can perform actions through API calls
- **Development Tools**: Build developer tools that can interact with APIs intelligently
