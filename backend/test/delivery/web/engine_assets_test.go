package web_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"openmars/test/testutil"
)

func TestStatic_EngineModulesPreserveWasmMIMEGzipAndCaching(t *testing.T) {
	root := t.TempDir()
	handler := newHandlerIn(t, root)
	for _, backend := range []string{"webgpu", "webgl2"} {
		t.Run(backend, func(t *testing.T) {
			// Valid empty modules with distinct gp/gl custom sections. The client
			// needs these binary bytes, including when compression is negotiated.
			module := []byte{0, 'a', 's', 'm', 1, 0, 0, 0, 0, 3, 2, 'g', backend[4]}
			name := "assets/engine-" + backend + "-abc123.wasm"
			full := filepath.Join(root, filepath.FromSlash(name))
			testutil.AssertNoError(t, os.WriteFile(full, module, 0o644), "write module")
			var compressed bytes.Buffer
			writer := gzip.NewWriter(&compressed)
			_, err := writer.Write(module)
			testutil.AssertNoError(t, err, "compress module")
			testutil.AssertNoError(t, writer.Close(), "finish compression")
			testutil.AssertNoError(t, os.WriteFile(full+".gz", compressed.Bytes(), 0o644), "write compressed module")

			for _, encoding := range []string{"identity", "gzip"} {
				t.Run(encoding, func(t *testing.T) {
					headers := map[string]string{"Accept-Encoding": encoding}
					response := serve(handler, http.MethodGet, "/"+name, headers)
					testutil.AssertEqual(t, http.StatusOK, response.Code, "status")
					testutil.AssertEqual(t, "application/wasm", response.Header().Get("Content-Type"), "streaming MIME")
					testutil.AssertEqual(t, "public, max-age=31536000, immutable", response.Header().Get("Cache-Control"), "hashed asset cache")
					testutil.AssertEqual(t, "Accept-Encoding", response.Header().Get("Vary"), "compression cache key")
					body := response.Body.Bytes()
					if encoding == "gzip" {
						testutil.AssertEqual(t, "gzip", response.Header().Get("Content-Encoding"), "gzip response")
						reader, err := gzip.NewReader(bytes.NewReader(body))
						testutil.AssertNoError(t, err, "read gzip response")
						body, err = io.ReadAll(reader)
						testutil.AssertNoError(t, err, "decode module")
						testutil.AssertNoError(t, reader.Close(), "close gzip reader")
					} else {
						testutil.AssertEqual(t, "", response.Header().Get("Content-Encoding"), "plain response")
					}
					if !bytes.Equal(body, module) {
						t.Fatal("module bytes changed during delivery")
					}
					etag := response.Header().Get("ETag")
					if etag == "" {
						t.Fatal("missing module validator")
					}
					headers["If-None-Match"] = etag
					cached := serve(handler, http.MethodGet, "/"+name, headers)
					testutil.AssertEqual(t, http.StatusNotModified, cached.Code, "conditional module request")
					testutil.AssertEqual(t, 0, cached.Body.Len(), "304 body")
				})
			}
		})
	}
}

func TestStatic_MissingEngineModuleDoesNotFallBackToHTML(t *testing.T) {
	handler := newHandler(t)
	for _, backend := range []string{"webgpu", "webgl2"} {
		response := serve(handler, http.MethodGet, "/assets/engine-"+backend+"-missing.wasm", nil)
		testutil.AssertEqual(t, http.StatusNotFound, response.Code, "missing module")
		if bytes.Contains(response.Body.Bytes(), []byte(shellBody)) {
			t.Fatal("missing module returned the application shell")
		}
	}
}
