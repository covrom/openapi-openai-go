package apiai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Execute API request
func executeAPIRequest(client *APIClient, fn *FunctionDefinition, args map[string]any) (any, error) {
	// Build URL with path parameters
	url := client.BaseURL + fn.OapiPath

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

	req, err := http.NewRequest(fn.OapiMethod, url, strings.NewReader(string(body)))
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
	return executeAPIRequest(client, fn, arguments)
}
