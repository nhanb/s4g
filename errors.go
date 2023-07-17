package main

import (
	"fmt"
	"html/template"
)

type UserFileErr struct {
	File  string
	Field string
	Msg   string
}

func (e *UserFileErr) Error() string {
	return fmt.Sprintf("UserFileErr - %s - %s: %s", e.File, e.Field, e.Msg)
}

func (e *UserFileErr) Html() template.HTML {
	return template.HTML(fmt.Sprintf(
		"<p>In file <b>%s</b>, field <b>%s</b>: %s </p>",
		e.File, e.Field, e.Msg,
	))
}
