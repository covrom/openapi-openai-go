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

	for _, pp := range fn.PathParams {
		u = strings.ReplaceAll(u, fmt.Sprintf("{%s}", pp), url.PathEscape(fmt.Sprint(args[pp])))
	}

	if len(fn.QueryParams) > 0 {
		uu, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		uq := uu.Query()
		for _, qp := range fn.QueryParams {
			uq.Set(qp, fmt.Sprint(args[qp]))
		}
		uu.RawQuery = uq.Encode()
		u = uu.String()
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
