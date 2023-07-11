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
const clientIdHeader = "Client-Id"

//go:embed livereload.html
var lrScript []byte

var pleaseReload = []byte("1")
var dontReload = []byte("0")

var state = struct {
	// Maps each client ID to whether they should reload on next ajax request.
	//
	// Client IDs are generated on client side so that an open tab's
	// livereload feature keeps working even when the server is restarted.
	clients map[string]bool
	mut     sync.RWMutex
}{
	clients: make(map[string]bool),
}

func init() {
	lrScript = bytes.ReplaceAll(
		lrScript, []byte("{{LR_ENDPOINT}}"), []byte(endpoint),
	)
	lrScript = bytes.ReplaceAll(
		lrScript, []byte("{{PLEASE_RELOAD}}"), pleaseReload,
	)
	lrScript = bytes.ReplaceAll(
		lrScript, []byte("{{CLIENT_ID_HEADER}}"), []byte(clientIdHeader),
	)
}

// For html pages, insert a script tag to enable livereload
func Middleware(fsys writablefs.FS, f http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Handle AJAX endpoint
		if path == endpoint {
			clientId := r.Header.Get(clientIdHeader)
			state.mut.RLock()
			shouldReload, ok := state.clients[clientId]
			state.mut.RUnlock()

			// New client: add client to state, don't reload
			if !ok {
				//fmt.Println("New livereload client:", clientId)
				state.mut.Lock()
				state.clients[clientId] = false
				state.mut.Unlock()
				w.Write(dontReload)
				return
			}

			// Existing client:
			if shouldReload {
				w.Write(pleaseReload)
				// On reload, the browser tab will generate another client ID,
				// so we can safely delete the old client ID now:
				state.mut.Lock()
				delete(state.clients, clientId)
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
	defer state.mut.Unlock()
	for k := range state.clients {
		state.clients[k] = true
	}
}

func withLiveReload(original []byte) []byte {
	bodyEndPos := bytes.LastIndex(original, []byte("</body>"))
	if bodyEndPos == -1 {
		// If the HTML is so malformed that it doesn't close its body,
		// then just append our livereload script at the end and hope
		// for the best.
		bodyEndPos = len(original)
	}
	result := make([]byte, len(original)+len(lrScript))
	copy(result, original[:bodyEndPos])
	copy(result[bodyEndPos:], lrScript)
	copy(result[bodyEndPos+len(lrScript):], original[bodyEndPos:])
	return result
}
