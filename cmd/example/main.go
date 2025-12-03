package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	apiai "github.com/covrom/openapi-openai-go"
	"github.com/openai/openai-go/v3"
)

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

	var spec apiai.OpenAPISpec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		log.Fatal(err)
	}

	// Convert to functions
	functions := apiai.ConvertOpenAPIToFunctions(&spec)

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

	// Example using OpenAI SDK with modern Tools API
	ctx := context.Background()
	client := openai.NewClient()

	question := "What pets are available with limit 5?"

	print("> ")
	println(question)

	// Prepare tools for OpenAI
	var tools []openai.ChatCompletionToolUnionParam
	for _, fn := range functions {
		// Convert parameters to the expected format
		params := fn.Parameters
		tools = append(tools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        fn.Name,
			Description: openai.String(fn.Description),
			Parameters: openai.FunctionParameters{
				"type":       params.Type,
				"properties": params.Properties,
				"required":   params.Required,
			},
		}))
	}

	// Make request to OpenAI
	completion, err := client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model: openai.ChatModelGPT4o,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(question),
			},
			Tools: tools,
			Seed:  openai.Int(0),
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	toolCalls := completion.Choices[0].Message.ToolCalls

	// Return early if there are no tool calls
	if len(toolCalls) == 0 {
		fmt.Printf("No function call")
		return
	}

	// If there was a function call, continue the conversation
	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(question),
		},
		Tools: tools,
		Seed:  openai.Int(0),
	}

	cli, err := apiai.NewAPIClient("https://petstore.swagger.io/v2", nil)
	if err != nil {
		log.Fatal(err)
	}

	params.Messages = append(params.Messages, completion.Choices[0].Message.ToParam())
	for _, toolCall := range toolCalls {
		// Parse arguments
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("Error parsing arguments: %v", err)
			continue
		}

		fn, ok := functions[toolCall.Function.Name]
		if !ok {
			log.Printf("Unknown function: %v", toolCall.Function.Name)
			continue
		}

		// Execute the function
		result, err := apiai.ExecuteFunction(cli, fn, args)
		if err != nil {
			log.Printf("Error executing function: %v", err)
			// Send error message back
			params.Messages = append(params.Messages, openai.ToolMessage(
				fmt.Sprintf("Error executing function: %v", err),
				toolCall.ID,
			))
		} else {
			fmt.Printf("Function %s result: %+v\n", toolCall.Function.Name, result)
			// Send result back to the model
			resultJSON, _ := json.Marshal(result)
			params.Messages = append(params.Messages, openai.ToolMessage(
				string(resultJSON),
				toolCall.ID,
			))
		}
	}

	// Get final response
	completion, err = client.Chat.Completions.New(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	println(completion.Choices[0].Message.Content)
}
