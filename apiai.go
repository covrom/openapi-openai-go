package apiai

import (
	"net/http"
	"net/url"
	"strings"
)

// APIClient handles HTTP requests to the API
type APIClient struct {
	BaseURL    *url.URL
	HTTPClient *http.Client // change this to client with authenticate
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string, authConfig *AuthConfig, opts ...func(*http.Client)) (*APIClient, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	cli := NewHTTPClient(authConfig, opts...)
	return &APIClient{
		BaseURL:    u,
		HTTPClient: cli,
	}, nil
}

// FunctionDefinition represents an LLM function definition
type FunctionDefinition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Parameters  Schema   `json:"parameters"`
	OapiMethod  string   `json:"-"`
	OapiPath    string   `json:"-"`
	PathParams  []string `json:"-"`
	QueryParams []string `json:"-"`
}

// OpenAPISpec represents an OpenAPI 3.x specification
type OpenAPISpec struct {
	OpenAPI    string              `json:"openapi"`
	Paths      map[string]PathItem `json:"paths"`
	Info       Info                `json:"info"`
	Components *Components         `json:"components,omitempty"`
}

// Info contains API metadata
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// Components contains reusable objects
type Components struct {
	Parameters map[string]Parameter `json:"parameters,omitempty"`
}

// PathItem represents a path in the OpenAPI spec
type PathItem struct {
	Get    *Operation `json:"get"`
	Post   *Operation `json:"post"`
	Put    *Operation `json:"put"`
	Delete *Operation `json:"delete"`
	Patch  *Operation `json:"patch"`
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

// Operation represents an API operation
type Operation struct {
	Summary     string       `json:"summary"`
	Description string       `json:"description"`
	Parameters  []Parameter  `json:"parameters,omitempty"`
	RequestBody *RequestBody `json:"requestBody,omitempty"`
}

// Parameter represents an API parameter
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description"`
	Required    bool    `json:"required"`
	Schema      *Schema `json:"schema"`
	Ref         string  `json:"$ref,omitempty"`
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
	Type        string             `json:"type"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Format      string             `json:"format,omitempty"`
	Description string             `json:"description,omitempty"`
}

// resolveParameterRef resolves a $ref parameter to its actual definition
func resolveParameterRef(param Parameter, spec *OpenAPISpec) Parameter {
	if param.Ref != "" && spec.Components != nil && spec.Components.Parameters != nil {
		// Extract parameter name from $ref, e.g., "#/components/parameters/offsetParam" -> "offsetParam"
		refParts := strings.Split(param.Ref, "/")
		if len(refParts) == 4 && refParts[0] == "#" && refParts[1] == "components" && refParts[2] == "parameters" {
			paramName := refParts[3]
			if referencedParam, exists := spec.Components.Parameters[paramName]; exists {
				// Return the referenced parameter
				return referencedParam
			}
		}
	}
	// Return the original parameter if no reference is found
	return param
}

// ConvertOpenAPIToFunctions converts OpenAPI spec to LLM function definitions
func ConvertOpenAPIToFunctions(spec *OpenAPISpec) map[string]*FunctionDefinition {
	functions := map[string]*FunctionDefinition{}

	for path, pathItem := range spec.Paths {
		for method, op := range pathItem.getOperations() {
			if op == nil {
				continue
			}

			desc := op.Summary
			if len(op.Description) > 0 {
				desc = desc + "\n" + op.Description
			}

			funcDef := &FunctionDefinition{
				Name:        sanitizeFunctionName(path, method),
				Description: desc,
				OapiMethod:  method,
				OapiPath:    path,
			}

			// Build parameters schema
			params := Schema{
				Type:       "object",
				Properties: map[string]*Schema{},
				Required:   []string{},
			}

			var pathParams, queryParams []string

			// Handle path and query parameters
			for _, param := range op.Parameters {
				// Resolve $ref if present
				resolvedParam := resolveParameterRef(param, spec)

				if resolvedParam.In == "path" || resolvedParam.In == "query" {
					prop := convertSchemaToProperty(resolvedParam.Schema)
					prop.Description = resolvedParam.Description
					params.Properties[resolvedParam.Name] = prop

					if resolvedParam.Required {
						params.Required = append(params.Required, resolvedParam.Name)
					}

					if resolvedParam.In == "path" {
						pathParams = append(pathParams, resolvedParam.Name)
					} else {
						queryParams = append(queryParams, resolvedParam.Name)
					}
				}
			}

			// Handle request body
			if op.RequestBody != nil {
				for _, mediaType := range op.RequestBody.Content {
					if mediaType.Schema != nil {
						reqBodySchema := convertSchemaToProperty(mediaType.Schema)
						params.Properties["requestBody"] = reqBodySchema
					}
				}
			}

			funcDef.Parameters = params
			funcDef.PathParams = pathParams
			funcDef.QueryParams = queryParams

			functions[funcDef.Name] = funcDef
		}
	}

	return functions
}

// Sanitize function name
func sanitizeFunctionName(path, method string) string {
	name := strings.ReplaceAll(path, "/", "_")
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")
	name = method + name
	return strings.ToLower(name)
}

// Convert OpenAPI schema to function parameter property
func convertSchemaToProperty(schema *Schema) *Schema {
	if schema == nil {
		return &Schema{Type: "string"}
	}

	prop := &Schema{
		Type:        schema.Type,
		Description: schema.Description,
	}

	if schema.Properties != nil {
		props := map[string]*Schema{}
		for name, subSchema := range schema.Properties {
			props[name] = convertSchemaToProperty(subSchema)
		}
		prop.Properties = props

		if len(schema.Required) > 0 {
			prop.Required = schema.Required
		}
	}

	return prop
}
