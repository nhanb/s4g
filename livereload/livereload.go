package livereload

import (
	"bytes"
	_ "embed"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"go.imnhan.com/webmaker2000/writablefs"
)

const endpoint = "/_livereload"

//go:embed livereload.html
var lrScript []byte

var pleaseReload = []byte("1")
var dontReload = []byte("0")

var state struct {
	shouldReload bool
	mut          sync.RWMutex
}

func init() {
	lrScript = bytes.ReplaceAll(lrScript, []byte("{{LR_ENDPOINT}}"), []byte(endpoint))
	lrScript = bytes.ReplaceAll(lrScript, []byte("{{SHOULD_RELOAD}}"), pleaseReload)
}

// For html pages, insert a script tag to enable livereload
func Middleware(fsys writablefs.FS, f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Handle AJAX endpoint
		if path == endpoint {
			state.mut.RLock()
			shouldReload := state.shouldReload
			state.mut.RUnlock()

			if shouldReload {
				w.Write(pleaseReload)
				state.mut.Lock()
				state.shouldReload = false
				state.mut.Unlock()
			} else {
				w.Write(dontReload)
			}
			return
		}

		// For non-html requests, fall through to default FileServer handler
		if !strings.HasSuffix(path, ".html") && !strings.HasSuffix(path, "/") {
			f.ServeHTTP(w, r)
			return
		}

		if strings.HasSuffix(path, "/") {
			path += "index.html"
		}

		// Filesystem access doesn't expect leading slash "/"
		path = strings.TrimPrefix(path, "/")

		originalContent, err := fs.ReadFile(fsys, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		w.Write(withLiveReload(originalContent))
	})
}

func Trigger() {
	state.mut.Lock()
	state.shouldReload = true
	state.mut.Unlock()
}

func withLiveReload(original []byte) []byte {
	bodyEndPos := bytes.LastIndex(original, []byte("</body>"))
	result := make([]byte, len(original)+len(lrScript))
	copy(result, original[:bodyEndPos])
	copy(result[bodyEndPos:], lrScript)
	copy(result[bodyEndPos+len(lrScript):], original[bodyEndPos:])
	return result
}
