package jsonpathfilter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type JSONPathFilter struct {
	HeaderName string `json:"header_name"`
}

func (jpf *JSONPathFilter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	capture := newResponseCapture(w)
	if err := next.ServeHTTP(capture, r); err != nil {
		return err
	}

	contentType := capture.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var responseBody map[string]interface{}
		if err := json.Unmarshal(capture.body.Bytes(), &responseBody); err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %v", err)
		}

		jsonPathQuery := r.Header.Get(jpf.HeaderName)
		var output interface{}
		if jsonPathQuery != "" {
			var err error
			output, err = applyJSONPathFilter(responseBody, jsonPathQuery)
			if err != nil {
				return err
			}
		} else {
			output = responseBody
		}

		h := w.Header()
		for k, v := range capture.Header() {
			h[k] = v
		}
		h.Set("Content-Type", "application/json")
		h.Del("Content-Length")
		w.WriteHeader(capture.status)

		return json.NewEncoder(w).Encode(output)
	}

	h := w.Header()
	for k, v := range capture.Header() {
		h[k] = v
	}
	w.WriteHeader(capture.status)
	_, err := w.Write(capture.body.Bytes())
	return err
}

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

type responseCapture struct {
	rw          http.ResponseWriter
	header      http.Header
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func newResponseCapture(rw http.ResponseWriter) *responseCapture {
	return &responseCapture{
		rw:     rw,
		header: make(http.Header),
		status: http.StatusOK,
	}
}

func (rc *responseCapture) Header() http.Header {
	return rc.header
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	if !rc.wroteHeader {
		rc.WriteHeader(http.StatusOK)
	}
	return rc.body.Write(b)
}

func (rc *responseCapture) WriteHeader(statusCode int) {
	if rc.wroteHeader {
		return
	}
	rc.status = statusCode
	rc.wroteHeader = true
}

func init() {
	caddy.RegisterModule(JSONPathFilter{})
	httpcaddyfile.RegisterHandlerDirective("jsonpath_filter", parseJSONPathFilter)
}

func parseJSONPathFilter(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	args := h.RemainingArgs()
	headerName := "X-JsonPath"
	if len(args) > 0 {
		headerName = args[0]
	}
	return &JSONPathFilter{
		HeaderName: headerName,
	}, nil
}

func (JSONPathFilter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.jsonpath_filter",
		New: func() caddy.Module { return new(JSONPathFilter) },
	}
}
