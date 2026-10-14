package web_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"openmars/internal/delivery/web"
	"openmars/test/testutil"
)

const shellBody = "<!doctype html><title>shell</title>"

var appJS = strings.Repeat("console.log('terraforming');\n", 50)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	return newHandlerIn(t, t.TempDir())
}

func newHandlerIn(t *testing.T, root string) http.Handler {
	t.Helper()
	write := func(name string, body []byte) {
		full := filepath.Join(root, filepath.FromSlash(name))
		testutil.AssertNoError(t, os.MkdirAll(filepath.Dir(full), 0o755), "mkdir")
		testutil.AssertNoError(t, os.WriteFile(full, body, 0o644), "write "+name)
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, err := zw.Write([]byte(appJS))
	testutil.AssertNoError(t, err, "gzip")
	testutil.AssertNoError(t, zw.Close(), "gzip close")

	write("index.html", []byte(shellBody))
	write("manifest.json", []byte(`{"name":"openmars"}`))
	write("assets/index-abc.js", []byte(appJS))
	write("assets/index-abc.js.gz", gz.Bytes())
	write("assets/textures/mars.webp", []byte("0123456789"))
	testutil.AssertNoError(t, os.WriteFile(filepath.Join(filepath.Dir(root), "secret.txt"), []byte("secret"), 0o644), "write outside")

	handler, err := web.NewStaticHandler(root)
	testutil.AssertNoError(t, err, "new static handler")
	return handler
}

func serve(handler http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestStatic_CacheControlPerPathClass(t *testing.T) {
	handler := newHandler(t)
	cases := map[string]string{
		"/":                    "no-cache",
		"/index.html":          "no-cache",
		"/game/abc":            "no-cache",
		"/assets/index-abc.js": "public, max-age=31536000, immutable",
		"/manifest.json":       "public, max-age=86400",
	}
	for target, want := range cases {
		response := serve(handler, http.MethodGet, target, nil)
		testutil.AssertEqual(t, http.StatusOK, response.Code, target+" status")
		testutil.AssertEqual(t, want, response.Header().Get("Cache-Control"), target+" cache-control")
	}
}

func TestStatic_FallsBackToShellForClientRoutes(t *testing.T) {
	response := serve(newHandler(t), http.MethodGet, "/game/abc?type=join", nil)
	testutil.AssertEqual(t, http.StatusOK, response.Code, "status")
	testutil.AssertEqual(t, shellBody, response.Body.String(), "body")
	testutil.AssertTrue(t, strings.HasPrefix(response.Header().Get("Content-Type"), "text/html"), "content type")
}

func TestStatic_MissingFilesAre404(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{"/assets/missing.js", "/assets/textures", "/missing.png"} {
		response := serve(handler, http.MethodGet, target, nil)
		testutil.AssertEqual(t, http.StatusNotFound, response.Code, target)
	}
}

func TestStatic_ServesGzipOnlyWhenAccepted(t *testing.T) {
	handler := newHandler(t)

	plain := serve(handler, http.MethodGet, "/assets/index-abc.js", nil)
	testutil.AssertEqual(t, "", plain.Header().Get("Content-Encoding"), "plain encoding")
	testutil.AssertEqual(t, appJS, plain.Body.String(), "plain body")
	testutil.AssertTrue(t, strings.Contains(plain.Header().Get("Vary"), "Accept-Encoding"), "plain vary")

	compressed := serve(handler, http.MethodGet, "/assets/index-abc.js", map[string]string{"Accept-Encoding": "br, gzip"})
	testutil.AssertEqual(t, "gzip", compressed.Header().Get("Content-Encoding"), "gzip encoding")
	testutil.AssertTrue(t, strings.HasPrefix(compressed.Header().Get("Content-Type"), "text/javascript"), "gzip content type")
	reader, err := gzip.NewReader(compressed.Body)
	testutil.AssertNoError(t, err, "gzip reader")
	decoded, err := io.ReadAll(reader)
	testutil.AssertNoError(t, err, "gzip read")
	testutil.AssertEqual(t, appJS, string(decoded), "decoded body")
	testutil.AssertNotEqual(t, plain.Header().Get("ETag"), compressed.Header().Get("ETag"), "etag per encoding")

	refused := serve(handler, http.MethodGet, "/assets/index-abc.js", map[string]string{"Accept-Encoding": "gzip;q=0"})
	testutil.AssertEqual(t, "", refused.Header().Get("Content-Encoding"), "q=0 refuses gzip")
}

func TestStatic_AllowsCrossOriginReads(t *testing.T) {
	response := serve(newHandler(t), http.MethodGet, "/index.html", map[string]string{"Origin": "https://openmars.app"})
	testutil.AssertEqual(t, "*", response.Header().Get("Access-Control-Allow-Origin"), "cors")
	testutil.AssertEqual(t, "nosniff", response.Header().Get("X-Content-Type-Options"), "nosniff")
}

func TestStatic_RevalidatesWithEtag(t *testing.T) {
	handler := newHandler(t)
	first := serve(handler, http.MethodGet, "/assets/index-abc.js", nil)
	etag := first.Header().Get("ETag")
	testutil.AssertNotEqual(t, "", etag, "etag")
	second := serve(handler, http.MethodGet, "/assets/index-abc.js", map[string]string{"If-None-Match": etag})
	testutil.AssertEqual(t, http.StatusNotModified, second.Code, "not modified")
}

func TestStatic_ServesRanges(t *testing.T) {
	response := serve(newHandler(t), http.MethodGet, "/assets/textures/mars.webp", map[string]string{"Range": "bytes=2-5"})
	testutil.AssertEqual(t, http.StatusPartialContent, response.Code, "status")
	testutil.AssertEqual(t, "2345", response.Body.String(), "range body")
}

func TestStatic_HeadOmitsBody(t *testing.T) {
	response := serve(newHandler(t), http.MethodHead, "/index.html", nil)
	testutil.AssertEqual(t, http.StatusOK, response.Code, "status")
	testutil.AssertEqual(t, 0, response.Body.Len(), "body length")
}

func TestStatic_RejectsWrites(t *testing.T) {
	response := serve(newHandler(t), http.MethodPost, "/index.html", nil)
	testutil.AssertEqual(t, http.StatusMethodNotAllowed, response.Code, "status")
	testutil.AssertEqual(t, "GET, HEAD", response.Header().Get("Allow"), "allow")
}

func TestStatic_StaysInsideTheDirectory(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{"/../secret.txt", "/assets/../../secret.txt", "/%2e%2e/secret.txt"} {
		response := serve(handler, http.MethodGet, target, nil)
		testutil.AssertTrue(t, !strings.Contains(response.Body.String(), "secret"), target+" must not leak")
	}
}

func TestStatic_RequiresIndexHTML(t *testing.T) {
	_, err := web.NewStaticHandler(t.TempDir())
	testutil.AssertErrorContains(t, err, "has no index.html", "missing index.html")
}

func TestStatic_ServesRebuiltFrontendWithoutRestart(t *testing.T) {
	root := t.TempDir()
	handler := newHandlerIn(t, root)
	initial := serve(handler, http.MethodGet, "/cards?cId=097", nil)
	testutil.AssertEqual(t, shellBody, initial.Body.String(), "initial shell")

	newShell := `<!doctype html><script src="/assets/index-rebuilt.js"></script>`
	newJS := "console.log('rebuilt');"
	testutil.AssertNoError(t, os.RemoveAll(filepath.Join(root, "assets")), "remove old build assets")
	testutil.AssertNoError(t, os.Mkdir(filepath.Join(root, "assets"), 0o755), "recreate assets")
	testutil.AssertNoError(t, os.WriteFile(filepath.Join(root, "assets/index-rebuilt.js"), []byte(newJS), 0o644), "write rebuilt script")
	testutil.AssertNoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte(newShell), 0o644), "replace shell")

	updated := serve(handler, http.MethodGet, "/cards?cId=097", map[string]string{"If-None-Match": initial.Header().Get("ETag")})
	testutil.AssertEqual(t, http.StatusOK, updated.Code, "updated shell is not cached as unchanged")
	testutil.AssertEqual(t, newShell, updated.Body.String(), "rebuilt shell")
	testutil.AssertNotEqual(t, initial.Header().Get("ETag"), updated.Header().Get("ETag"), "updated shell etag")

	script := serve(handler, http.MethodGet, "/assets/index-rebuilt.js", nil)
	testutil.AssertEqual(t, http.StatusOK, script.Code, "new script status")
	testutil.AssertEqual(t, newJS, script.Body.String(), "new script body")
	testutil.AssertEqual(t, http.StatusNotFound, serve(handler, http.MethodGet, "/assets/index-abc.js", nil).Code, "removed script status")
}

func TestStatic_RefreshesCompressedSiblings(t *testing.T) {
	root := t.TempDir()
	handler := newHandlerIn(t, root)
	headers := map[string]string{"Accept-Encoding": "gzip"}
	initial := serve(handler, http.MethodGet, "/assets/index-abc.js", headers)
	testutil.AssertEqual(t, "gzip", initial.Header().Get("Content-Encoding"), "initial gzip")
	testutil.AssertNoError(t, os.Remove(filepath.Join(root, "assets/index-abc.js.gz")), "remove gzip")
	plain := serve(handler, http.MethodGet, "/assets/index-abc.js", headers)
	testutil.AssertEqual(t, http.StatusOK, plain.Code, "uncompressed status after removal")
	testutil.AssertEqual(t, "", plain.Header().Get("Content-Encoding"), "uncompressed after removal")
	testutil.AssertEqual(t, appJS, plain.Body.String(), "uncompressed body")

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte(shellBody))
	testutil.AssertNoError(t, err, "compress shell")
	testutil.AssertNoError(t, writer.Close(), "finish compression")
	testutil.AssertNoError(t, os.WriteFile(filepath.Join(root, "index.html.gz"), compressed.Bytes(), 0o644), "add gzip after startup")
	updated := serve(handler, http.MethodGet, "/cards", headers)
	testutil.AssertEqual(t, "gzip", updated.Header().Get("Content-Encoding"), "new compressed sibling")
	testutil.AssertEqual(t, compressed.String(), updated.Body.String(), "compressed body")
}

func TestStatic_DoesNotFollowSymlinksOutsideBuild(t *testing.T) {
	root := t.TempDir()
	handler := newHandlerIn(t, root)
	secret := filepath.Join(filepath.Dir(root), "secret.txt")
	testutil.AssertNoError(t, os.Symlink(secret, filepath.Join(root, "assets/leak.txt")), "symlink outside build")
	response := serve(handler, http.MethodGet, "/assets/leak.txt", nil)
	testutil.AssertEqual(t, http.StatusNotFound, response.Code, "outside symlink status")
}
