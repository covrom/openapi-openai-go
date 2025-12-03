package apiai

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
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
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Parameters  Schema   `json:"parameters" yaml:"parameters"`
	OapiMethod  string   `json:"-" yaml:"-"`
	OapiPath    string   `json:"-" yaml:"-"`
	PathParams  []string `json:"-" yaml:"-"`
	QueryParams []string `json:"-" yaml:"-"`
}

// OpenAPISpec represents an OpenAPI 3.x specification
type OpenAPISpec struct {
	OpenAPI    string              `json:"openapi" yaml:"openapi"`
	Paths      map[string]PathItem `json:"paths" yaml:"paths"`
	Info       Info                `json:"info" yaml:"info"`
	Components *Components         `json:"components,omitempty" yaml:"components,omitempty"`
}

// Info contains API metadata
type Info struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description" yaml:"description"`
	Version     string `json:"version" yaml:"version"`
}

// Components contains reusable objects
type Components struct {
	Parameters map[string]Parameter `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// PathItem represents a path in the OpenAPI spec
type PathItem struct {
	Get    *Operation `json:"get" yaml:"get"`
	Post   *Operation `json:"post" yaml:"post"`
	Put    *Operation `json:"put" yaml:"put"`
	Delete *Operation `json:"delete" yaml:"delete"`
	Patch  *Operation `json:"patch" yaml:"patch"`
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
	Summary     string       `json:"summary" yaml:"summary"`
	Description string       `json:"description" yaml:"description"`
	Parameters  []Parameter  `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBody `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
}

// Parameter represents an API parameter
type Parameter struct {
	Name        string  `json:"name" yaml:"name"`
	In          string  `json:"in" yaml:"in"`
	Description string  `json:"description" yaml:"description"`
	Required    bool    `json:"required" yaml:"required"`
	Schema      *Schema `json:"schema" yaml:"schema"`
	Ref         string  `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// RequestBody represents the request body definition
type RequestBody struct {
	Content map[string]MediaType `json:"content" yaml:"content"`
}

// MediaType represents media type in request/response body
type MediaType struct {
	Schema *Schema `json:"schema" yaml:"schema"`
}

// Schema represents JSON Schema
type Schema struct {
	Type        string             `json:"type" yaml:"type"`
	Properties  map[string]*Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string           `json:"required,omitempty" yaml:"required,omitempty"`
	Format      string             `json:"format,omitempty" yaml:"format,omitempty"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
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

// UnmarshalOpenAPISpecFromJSON unmarshals OpenAPI spec from JSON data
func UnmarshalOpenAPISpecFromJSON(data []byte) (*OpenAPISpec, error) {
	var spec OpenAPISpec
	err := json.Unmarshal(data, &spec)
	if err != nil {
		return nil, err
	}
	return &spec, nil
}

// UnmarshalOpenAPISpecFromYAML unmarshals OpenAPI spec from YAML data
func UnmarshalOpenAPISpecFromYAML(data []byte) (*OpenAPISpec, error) {
	var spec OpenAPISpec
	err := yaml.Unmarshal(data, &spec)
	if err != nil {
		return nil, err
	}
	return &spec, nil
}

// UnmarshalOpenAPISpec unmarshals OpenAPI spec from data, automatically detecting format (JSON or YAML)
func UnmarshalOpenAPISpec(data []byte) (*OpenAPISpec, error) {
	// Try JSON first
	var spec OpenAPISpec
	err := json.Unmarshal(data, &spec)
	if err != nil {
		// If JSON fails, try YAML
		err = yaml.Unmarshal(data, &spec)
		if err != nil {
			return nil, err
		}
	}
	return &spec, nil
}
