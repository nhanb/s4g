package livereload

import (
	"bytes"
	_ "embed"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"text/template"

	"go.imnhan.com/webmaker2000/writablefs"
)

const endpoint = "/_livereload"
const clientIdHeader = "Client-Id"

//go:embed livereload.html
var lrScript []byte

//go:embed error.html
var errorTmpl string

var pleaseReload = []byte("1")
var dontReload = []byte("0")

var state = struct {
	// Maps each client ID to whether they should reload on next ajax request.
	//
	// Client IDs are generated on client side so that an open tab's
	// livereload feature keeps working even when the server is restarted.
	clients    map[string]bool
	clientsMut sync.RWMutex
	err        error
	errMut     sync.RWMutex
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

func handleFunc(w http.ResponseWriter, r *http.Request) {
	clientId := r.Header.Get(clientIdHeader)
	state.clientsMut.RLock()
	shouldReload, ok := state.clients[clientId]
	state.clientsMut.RUnlock()

	// New client: add client to state, don't reload
	if !ok {
		//fmt.Println("New livereload client:", clientId)
		state.clientsMut.Lock()
		state.clients[clientId] = false
		state.clientsMut.Unlock()
		w.Write(dontReload)
		return
	}

	// Existing client:
	if shouldReload {
		w.Write(pleaseReload)
		// On reload, the browser tab will generate another client ID,
		// so we can safely delete the old client ID now:
		state.clientsMut.Lock()
		delete(state.clients, clientId)
		state.clientsMut.Unlock()
	} else {
		w.Write(dontReload)
	}
}

// For html pages, insert a script tag to enable livereload
func Middleware(mux *http.ServeMux, root string, fsys writablefs.FS, f http.Handler) http.Handler {

	// Handle AJAX endpoint
	mux.HandleFunc(endpoint, handleFunc)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		state.errMut.RLock()
		err = state.err
		state.errMut.RUnlock()
		if err != nil {
			serveError(w, r, err)
			return
		}

		path := r.URL.Path

		// For non-html requests, fall through to default FileServer handler
		if !strings.HasSuffix(path, ".html") && !strings.HasSuffix(path, "/") {
			f.ServeHTTP(w, r)
			return
		}

		filePath := path

		if strings.HasSuffix(filePath, "/") {
			filePath += "index.html"
		}

		filePath = strings.TrimPrefix(filePath, root)

		originalContent, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			f.ServeHTTP(w, r)
			return
		}

		w.Write(withLiveReload(originalContent))
	})
}

// Tell current browser tabs to reload
func Trigger() {
	state.clientsMut.Lock()
	defer state.clientsMut.Unlock()
	for k := range state.clients {
		state.clients[k] = true
	}
}

// When a non-nil error is set, the local webserver returns
// the error page for every path (except livereload duh).
func SetError(err error) {
	state.errMut.Lock()
	state.err = err
	state.errMut.Unlock()
	Trigger()
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

var errTmpl = template.Must(template.New("error").Parse(errorTmpl))

func serveError(w http.ResponseWriter, r *http.Request, err error) {
	var buf bytes.Buffer
	errTmpl.Execute(&buf, err.Error())
	body := withLiveReload(buf.Bytes())
	w.Write(body)
}
