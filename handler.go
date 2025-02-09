package jsonpathfilter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

// JSONPathFilter implements a reverse_proxy response modifier that applies a JSONPath filter.
// It reads the JSONPath query from a header on the original request (default "X-JsonPath").
type JSONPathFilter struct {
	// HeaderName is the name of the header to look for on the original request.
	// If not set, it defaults to "X-JsonPath".
	HeaderName string `json:"header_name,omitempty"`
}

// ModifyResponse implements the reverse_proxy response modifier interface.
func (jpf *JSONPathFilter) ModifyResponse(res *http.Response) error {
	// Process only JSON responses.
	contentType := res.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil
	}

	// Read and close the existing response body.
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	res.Body.Close()

	// Unmarshal the response JSON.
	var responseBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Get the JSONPath query from the original request's header.
	var jsonPathQuery string
	if res.Request != nil {
		jsonPathQuery = res.Request.Header.Get(jpf.HeaderName)
	}

	// If a query is provided, apply it.
	output := responseBody
	if jsonPathQuery != "" {
		output, err = applyJSONPathFilter(responseBody, jsonPathQuery)
		if err != nil {
			return fmt.Errorf("JSONPath filter error: %v", err)
		}
	}

	// Marshal the (possibly filtered) result back into JSON.
	newBodyBytes, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Update headers and reset the response body.
	res.Header.Set("Content-Type", "application/json")
	res.Header.Del("Content-Length")
	res.Body = io.NopCloser(bytes.NewReader(newBodyBytes))
	return nil
}

// applyJSONPathFilter applies the JSONPath query to data.
func applyJSONPathFilter(data map[string]interface{}, query string) (map[string]interface{}, error) {
	result, err := jsonpath.Get(query, data)
	if err != nil {
		return nil, fmt.Errorf("JSONPath query failed: %v", err)
	}
	filteredData, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("JSONPath query did not return an object")
	}
	return filteredData, nil
}

// UnmarshalCaddyfile configures JSONPathFilter from Caddyfile tokens.
// It allows an optional argument to override the default header ("X-JsonPath").
func (jpf *JSONPathFilter) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	// Set default header name.
	jpf.HeaderName = "X-JsonPath"

	// Allow an override: e.g., "jsonpath_filter My-Header" will use "My-Header".
	for d.Next() {
		if d.NextArg() {
			jpf.HeaderName = d.Val()
		}
	}
	return nil
}

// CaddyModule returns the Caddy module information.
// Note the module ID is namespaced under reverse_proxy.response_modifiers,
// meaning it can only be used within a reverse_proxy block.
func (JSONPathFilter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.reverse_proxy.response_modifiers.jsonpath_filter",
		New: func() caddy.Module { return new(JSONPathFilter) },
	}
}

func init() {
	caddy.RegisterModule(JSONPathFilter{})
}
