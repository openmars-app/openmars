// Package web serves the built frontend from disk next to the API.
package web

import (
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"openmars/internal/logger"
)

const (
	shellFile    = "index.html"
	assetsPrefix = "assets/"

	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
	cacheDefault    = "public, max-age=86400"

	// Covers the largest files (a 130 MB skybox) on slow links; the server-wide
	// WriteTimeout is sized for API responses.
	fileWriteDeadline = 10 * time.Minute
)

// Alpine has no mime.types, so Go's built-in table is all there is; these fill its gaps.
var contentTypes = map[string]string{
	".webmanifest": "application/manifest+json",
	".ico":         "image/x-icon",
	".map":         "application/json",
	".exr":         "image/x-exr",
	".glb":         "model/gltf-binary",
	".txt":         "text/plain; charset=utf-8",
}

type staticHandler struct {
	root string
}

// NewStaticHandler serves the current frontend build from disk.
// Nothing is held in memory: the frontend build is hundreds of megabytes.
// Precompressed "<file>.gz" siblings are served to clients that accept gzip.
func NewStaticHandler(dir string) (http.Handler, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	handler := &staticHandler{root: root}
	shell, _, err := handler.open(shellFile)
	if err != nil {
		return nil, fmt.Errorf("%s has no %s: %w", root, shellFile, err)
	}
	if err := shell.Close(); err != nil {
		return nil, err
	}
	logger.Get().Info("Serving frontend", slog.String("dir", root))
	return handler, nil
}

func (h *staticHandler) open(name string) (*os.File, fs.FileInfo, error) {
	file, err := os.OpenInRoot(h.root, name)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, fs.ErrNotExist
	}
	return file, info, nil
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if strings.HasSuffix(name, ".gz") {
		http.NotFound(w, r)
		return
	}
	f, info, err := h.open(name)
	if err != nil {
		// Unknown paths are client-side routes, but a missing file must not get the shell.
		if strings.HasPrefix(name, assetsPrefix) || path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		name = shellFile
		f, info, err = h.open(name)
		if err != nil {
			http.Error(w, "file unavailable", http.StatusInternalServerError)
			return
		}
	}
	defer func() { _ = f.Close() }()

	// Rebuilds replace both files and compressed siblings; use current metadata.
	gzipped := false
	if acceptsGzip(r) {
		if compressed, compressedInfo, err := h.open(name + ".gz"); err == nil {
			_ = f.Close()
			f, info = compressed, compressedInfo
			gzipped = true
		}
	}

	header := w.Header()
	// The openmars.app gateway boots this frontend from its own origin.
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", cacheControlFor(name))
	header.Set("Content-Type", contentTypeFor(name))
	header.Add("Vary", "Accept-Encoding")

	etag := fmt.Sprintf(`"%x-%x"`, info.Size(), info.ModTime().UnixNano())
	if gzipped {
		etag = fmt.Sprintf(`"%x-%x-gz"`, info.Size(), info.ModTime().UnixNano())
		header.Set("Content-Encoding", "gzip")
		// Ranges would address the compressed bytes, not the file the client asked for.
		r.Header.Del("Range")
	}
	header.Set("ETag", etag)

	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(fileWriteDeadline)); err != nil {
		logger.Get().Debug("Could not extend write deadline", slog.Any("error", err))
	}
	http.ServeContent(w, r, "", info.ModTime(), f)
}

func contentTypeFor(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if contentType, ok := contentTypes[ext]; ok {
		return contentType
	}
	if contentType := mime.TypeByExtension(ext); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func cacheControlFor(name string) string {
	switch {
	case name == shellFile:
		return cacheRevalidate
	case strings.HasPrefix(name, assetsPrefix):
		return cacheImmutable
	default:
		return cacheDefault
	}
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		encoding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(encoding), "gzip") && !strings.Contains(strings.ReplaceAll(params, " ", ""), "q=0") {
			return true
		}
	}
	return false
}
