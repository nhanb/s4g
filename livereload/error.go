package livereload

import (
	"bytes"
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed error.html
var errorTmpl string

var errTmpl = template.Must(template.New("error").Parse(errorTmpl))

// Error that has a user-friendly HTML representation.
type htmlErr interface {
	error
	Html() template.HTML
}

func serveError(w http.ResponseWriter, r *http.Request, err htmlErr) {
	var buf bytes.Buffer
	errTmpl.Execute(&buf, err)
	body := withLiveReload(buf.Bytes())
	w.Write(body)
}
