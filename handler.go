package jsonpathfilter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/PaesslerAG/jsonpath"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyhttp"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func init() {
	caddy.RegisterModule(ResponseFilter{})
}

// ResponseFilter filters JSON responses using the "jsonpath_filter" query parameter.
type ResponseFilter struct{}

func (ResponseFilter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.jsonpath_filter",
		New: func() caddy.Module { return new(ResponseFilter) },
	}
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m *ResponseFilter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// Capture response
	rec := caddyhttp.NewResponseRecorder(w, nil, func(status int, hdr http.Header) bool { return true })
	if err := next.ServeHTTP(rec, r); err != nil {
		return err
	}

	// Only handle JSON
	ct := rec.Header().Get("Content-Type")
	if ct == "" || ct != "application/json" {
		_, err := w.Write(rec.Body())
		return err
	}

	// Parse JSON
	var data interface{}
	if err := json.Unmarshal(rec.Body(), &data); err != nil {
		// Not JSON, return original
		_, err := w.Write(rec.Body())
		return err
	}

	// Get JSONPath expression from query param
	expr := r.URL.Query().Get("jsonpath_filter")
	if expr == "" {
		// No query param, return original JSON
		_, err := w.Write(rec.Body())
		return err
	}

	// Apply JSONPath
	result, err := jsonpath.Get(expr, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("JSONPath error: %v", err), http.StatusBadRequest)
		return nil
	}

	// Marshal filtered result
	filtered, err := json.Marshal(result)
	if err != nil {
		return err
	}

	// Write filtered response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(rec.Status())
	_, err = io.Copy(w, bytes.NewReader(filtered))
	return err
}

// UnmarshalCaddyfile is no-op since we don't require any config
func (m *ResponseFilter) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	return nil
}

// Interface guards
var (
	_ caddyhttp.MiddlewareHandler = (*ResponseFilter)(nil)
	_ caddyfile.Unmarshaler       = (*ResponseFilter)(nil)
)
