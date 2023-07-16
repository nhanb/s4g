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

func serveError(w http.ResponseWriter, r *http.Request, err error) {
	var buf bytes.Buffer
	_, ok := err.(htmlErr)
	if ok {
		errTmpl.Execute(&buf, err)
	} else {
		// Shim for errors that don't support HTML output
		errTmpl.Execute(&buf, struct {
			Error string
			Html  template.HTML
		}{
			Error: err.Error(),
			Html:  template.HTML(err.Error()),
		})
	}
	body := withLiveReload(buf.Bytes())
	w.Write(body)
}
