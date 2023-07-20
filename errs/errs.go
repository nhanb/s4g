package errs

import (
	"fmt"
	"html/template"
)

// Represents a user input error, which in webmaker2000's case is almost
// always some malformed file.
type UserErr struct {
	File string
	Msg  string

	// Optional. Zero value means unavailable.
	Line int

	// Optional. Zero value means unavailable.
	Column int

	// Optional. Zero value means unavailable.
	Field string
}

func (e *UserErr) Error() string {
	return fmt.Sprintf(
		"UserFileErr: %s - %d:%d:%s %s",
		e.File, e.Line, e.Column, e.Field, e.Msg,
	)
}

func (e *UserErr) Html() template.HTML {
	content := fmt.Sprintf("In file <b>%s</b>", e.File)
	if e.Line != 0 {
		content += fmt.Sprintf(", line %d", e.Line)
	}
	if e.Column != 0 {
		content += fmt.Sprintf(", column %d", e.Column)
	}
	if e.Field != "" {
		content += fmt.Sprintf(", field <b>%s</b>", e.Field)
	}

	content = fmt.Sprintf("<p>%s: %s</p>", content, e.Msg)
	return template.HTML(content)
}
