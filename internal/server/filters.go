package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/mostlygeek/llama-swap/internal/chain"
	"github.com/mostlygeek/llama-swap/internal/config"
	"github.com/mostlygeek/llama-swap/internal/shared"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// CreateFilterMiddleware returns middleware that applies per-model request-body
// filters to JSON requests before they are forwarded upstream:
//
//   - UseModelName rewrite (issue #69)
//   - StripParams removal (issue #174)
//   - SetParams injection (issue #453)
//   - SetParamsByID per-alias overrides
//
// Non-JSON requests (GET, multipart forms) pass through untouched. The buffered
// body is re-attached with Content-Length / Transfer-Encoding cleanup so the
// downstream reverse proxy forwards the correct bytes (see issue #11).
func CreateFilterMiddleware(cfg config.Config) chain.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				next.ServeHTTP(w, r)
				return
			}

			data, err := shared.FetchContext(r, cfg)
			if err != nil {
				shared.SendError(w, r, shared.ErrNoModelInContext)
				return
			}

			useModelName, filters, ok := resolveFilters(cfg, data.Model)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				shared.SendResponse(w, r, http.StatusBadRequest, "could not read request body")
				return
			}

			body, err = applyFilters(body, data.Model, useModelName, filters)
			if err != nil {
				shared.SendResponse(w, r, http.StatusInternalServerError, err.Error())
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			r.Header.Del("Transfer-Encoding")
			r.Header.Set("Content-Length", strconv.Itoa(len(body)))
			r.ContentLength = int64(len(body))

			next.ServeHTTP(w, r)
		})
	}
}

// CreateFormFilterMiddleware returns middleware that applies the UseModelName
// rewrite (issue #69) to multipart/form-data requests before they are forwarded
// upstream. JSON-body filters (StripParams, SetParams) do not apply to form
// endpoints; only the "model" field is rewritten.
//
// Non-multipart requests pass through untouched. When a rewrite is needed the
// form is reconstructed and re-attached with Content-Type / Content-Length
// cleanup so the downstream reverse proxy forwards the correct bytes.
func CreateFormFilterMiddleware(cfg config.Config) chain.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
				next.ServeHTTP(w, r)
				return
			}

			data, err := shared.FetchContext(r, cfg)
			if err != nil {
				shared.SendError(w, r, shared.ErrNoModelInContext)
				return
			}

			useModelName, _, ok := resolveFilters(cfg, data.Model)
			if !ok || useModelName == "" {
				next.ServeHTTP(w, r)
				return
			}

			if err := r.ParseMultipartForm(32 << 20); err != nil {
				shared.SendResponse(w, r, http.StatusBadRequest, fmt.Sprintf("error parsing multipart form: %s", err.Error()))
				return
			}

			body, contentType, err := rewriteMultipartModel(r.MultipartForm, useModelName)
			if err != nil {
				shared.SendResponse(w, r, http.StatusInternalServerError, err.Error())
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			r.MultipartForm = nil
			r.Header.Del("Transfer-Encoding")
			r.Header.Set("Content-Type", contentType)
			r.Header.Set("Content-Length", strconv.Itoa(len(body)))
			r.ContentLength = int64(len(body))

			next.ServeHTTP(w, r)
		})
	}
}

// rewriteMultipartModel reconstructs a multipart form, replacing the "model"
// field value with useModelName. It returns the encoded body and the matching
// Content-Type header (which carries the generated boundary).
func rewriteMultipartModel(form *multipart.Form, useModelName string) ([]byte, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	for key, values := range form.Value {
		for _, value := range values {
			if key == "model" {
				value = useModelName
			}
			field, err := mw.CreateFormField(key)
			if err != nil {
				return nil, "", fmt.Errorf("error recreating form field %s: %w", key, err)
			}
			if _, err := field.Write([]byte(value)); err != nil {
				return nil, "", fmt.Errorf("error writing form field %s: %w", key, err)
			}
		}
	}

	for key, headers := range form.File {
		for _, fh := range headers {
			part, err := mw.CreateFormFile(key, fh.Filename)
			if err != nil {
				return nil, "", fmt.Errorf("error recreating form file %s: %w", key, err)
			}
			file, err := fh.Open()
			if err != nil {
				return nil, "", fmt.Errorf("error opening uploaded file %s: %w", key, err)
			}
			if _, err := io.Copy(part, file); err != nil {
				file.Close()
				return nil, "", fmt.Errorf("error copying file data %s: %w", key, err)
			}
			file.Close()
		}
	}

	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("error finalizing multipart form: %w", err)
	}
	return buf.Bytes(), mw.FormDataContentType(), nil
}

// resolveFilters returns the filter settings for a requested model. UseModelName
// only applies to local models; peers carry filters but no name rewrite.
func resolveFilters(cfg config.Config, requested string) (useModelName string, filters config.Filters, ok bool) {
	if realName, found := cfg.RealModelName(requested); found {
		mc := cfg.Models[realName]
		return mc.UseModelName, mc.Filters.Filters, true
	}
	for _, peer := range cfg.Peers {
		for _, m := range peer.Models {
			if m == requested {
				return "", peer.Filters, true
			}
		}
	}
	return "", config.Filters{}, false
}

// applyFilters rewrites the JSON body in place. Order matches the legacy
// ProxyManager: useModelName, stripParams, setParams, then setParamsByID (which
// can override setParams).
func applyFilters(body []byte, requested, useModelName string, f config.Filters) ([]byte, error) {
	var err error

	if useModelName != "" {
		if body, err = sjson.SetBytes(body, "model", useModelName); err != nil {
			return nil, fmt.Errorf("error rewriting model name in JSON: %w", err)
		}
	}

	for _, param := range f.SanitizedStripParams() {
		if body, err = sjson.DeleteBytes(body, param); err != nil {
			return nil, fmt.Errorf("error stripping parameter %s from request", param)
		}
	}

	setParams, setKeys := f.SanitizedSetParams()
	for _, key := range setKeys {
		if body, err = sjson.SetBytes(body, key, setParams[key]); err != nil {
			return nil, fmt.Errorf("error setting parameter %s in request", key)
		}
	}

	byID, byIDKeys := f.SanitizedSetParamsByID(requested)
	for _, key := range byIDKeys {
		if body, err = sjson.SetBytes(body, key, byID[key]); err != nil {
			return nil, fmt.Errorf("error setting parameter %s in request", key)
		}
	}

	if allow := f.SanitizedAllowTools(); len(allow) > 0 {
		if body, err = filterTools(body, allow); err != nil {
			return nil, err
		}
	}

	return body, nil
}

// filterTools drops every tool in the request whose function name is not in
// allow, matched case-insensitively. Tool lists are supplied by the client, so
// this is the only place a server-side policy on them can be enforced.
//
// Requests carrying no tools array are left untouched. When filtering empties
// the list, "tools" and "tool_choice" are removed outright rather than left as
// an empty array, which some upstreams reject. A "tool_choice" pinning a tool
// that was just removed is reset to "auto" for the same reason.
func filterTools(body []byte, allow []string) ([]byte, error) {
	tools := gjson.GetBytes(body, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return body, nil
	}
	original := tools.Array()
	if len(original) == 0 {
		return body, nil
	}

	allowed := make(map[string]bool, len(allow))
	for _, name := range allow {
		allowed[strings.ToLower(name)] = true
	}

	kept := make([]json.RawMessage, 0, len(original))
	keptNames := make(map[string]bool, len(original))
	for _, tool := range original {
		// OpenAI nests the name under "function"; the Anthropic-style
		// /v1/messages payload puts it at the top level.
		name := tool.Get("function.name").String()
		if name == "" {
			name = tool.Get("name").String()
		}
		if allowed[strings.ToLower(name)] {
			kept = append(kept, json.RawMessage(tool.Raw))
			keptNames[name] = true
		}
	}

	if len(kept) == len(original) {
		return body, nil
	}

	var err error
	if len(kept) == 0 {
		if body, err = sjson.DeleteBytes(body, "tools"); err != nil {
			return nil, fmt.Errorf("error removing tools from request: %w", err)
		}
		if body, err = sjson.DeleteBytes(body, "tool_choice"); err != nil {
			return nil, fmt.Errorf("error removing tool_choice from request: %w", err)
		}
		return body, nil
	}

	raw, err := json.Marshal(kept)
	if err != nil {
		return nil, fmt.Errorf("error re-encoding filtered tools: %w", err)
	}
	if body, err = sjson.SetRawBytes(body, "tools", raw); err != nil {
		return nil, fmt.Errorf("error writing filtered tools: %w", err)
	}

	if pinned := gjson.GetBytes(body, "tool_choice.function.name"); pinned.Exists() && !keptNames[pinned.String()] {
		if body, err = sjson.SetBytes(body, "tool_choice", "auto"); err != nil {
			return nil, fmt.Errorf("error resetting tool_choice: %w", err)
		}
	}

	return body, nil
}
