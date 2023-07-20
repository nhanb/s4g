package livereload

import (
	"bytes"
	_ "embed"
	"errors"
	"html/template"
	"net/http"

	"go.imnhan.com/webmaker2000/errs"
)

//go:embed error.html
var errorTmpl string

var errTmpl = template.Must(template.New("error").Parse(errorTmpl))

type errTmplInput struct {
	Text string
	Html template.HTML
}

func serveError(w http.ResponseWriter, r *http.Request, e error) {
	var buf bytes.Buffer
	var uerr *errs.UserErr
	ok := errors.As(e, &uerr)

	var tmplInput errTmplInput
	if ok {
		tmplInput.Text = uerr.Error()
		tmplInput.Html = uerr.Html()
	} else {
		tmplInput.Text = e.Error()
		tmplInput.Html = template.HTML(e.Error())
	}
	err := errTmpl.Execute(&buf, tmplInput)
	if err != nil {
		panic(err)
	}
	body := withLiveReload(buf.Bytes())
	w.Write(body)
}
