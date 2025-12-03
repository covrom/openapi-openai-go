package apiai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Execute API request
func executeAPIRequest(client *APIClient, fn *FunctionDefinition, requestBody any, args map[string]any) (any, error) {
	// Build URL with path parameters
	u := client.BaseURL.JoinPath(fn.OapiPath).String()

	for key, value := range args {
		u = strings.ReplaceAll(u, fmt.Sprintf("{%s}", key), url.PathEscape(fmt.Sprint(value)))
	}

	var body []byte
	var err error

	if requestBody != nil {
		body, err = json.Marshal(requestBody)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(fn.OapiMethod, u, bytes.NewReader(body))
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

	var result any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// ExecuteFunction calls the registered function handler
func ExecuteFunction(client *APIClient, fn *FunctionDefinition, arguments map[string]any) (any, error) {
	requestBody := arguments["requestBody"]

	return executeAPIRequest(client, fn, requestBody, arguments)
}
