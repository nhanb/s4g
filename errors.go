package main

import (
	"fmt"
	"html/template"
)

type SiteMetadataErr struct {
	Field string
	Msg   string
}

func (e *SiteMetadataErr) Error() string {
	return fmt.Sprintf("SiteMetadataErr - %s: %s", e.Field, e.Msg)
}

func (e *SiteMetadataErr) Html() template.HTML {
	return template.HTML(fmt.Sprintf(
		"<p>In file <b>%s</b>, field <b>%s</b>: %s </p>",
		SiteFileName, e.Field, e.Msg,
	))
}
