package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.imnhan.com/s4g/errs"
	"go.imnhan.com/s4g/writablefs"
)

type SiteMetadata struct {
	Address       string
	Name          string
	Tagline       string
	Root          string
	ShowFooter    bool
	FooterText    template.HTML
	NavbarLinks   []string
	DefaultThumb  string
	AuthorName    string
	AuthorURI     string
	AuthorEmail   string
	AuthorTwitter string
}

type PageType int

const (
	PTPost PageType = iota
	PTHome
	PTSeriesIndex
	PTCustom
)

func (t PageType) String() string {
	switch t {
	case PTPost:
		return "post"
	case PTHome:
		return "home"
	case PTSeriesIndex:
		return "series-index"
	case PTCustom:
		return "custom"
	default:
		panic(fmt.Sprintf("unexpected value: %d", t))
	}
}

func ParsePageType(name string) (PageType, error) {
	switch name {
	case "post":
		return PTPost, nil
	case "home":
		return PTHome, nil
	case "series-index":
		return PTSeriesIndex, nil
	case "custom":
		return PTCustom, nil
	default:
		return -1, fmt.Errorf(`"%s" is not a valid PageType`, name)
	}
}

type ArticleMetadata struct {
	Title       string
	Description string
	IsDraft     bool
	PostedAt    time.Time
	PageType    PageType
	Templates   []string
	ShowInFeed  bool
	Thumb       string
}

func NewSiteMetadata() SiteMetadata {
	return SiteMetadata{
		Address:      "http://example.com",
		Name:         "This is my website",
		Tagline:      "and it's fine",
		Root:         "/",
		ShowFooter:   true,
		FooterText:   `Made with <a href="https://github.com/nhanb/s4g">s4g</a>`,
		NavbarLinks:  []string{"index.dj", "#s4g#https://github.com/nhanb/s4g"},
		DefaultThumb: "",

		AuthorName:    "Scoop Newsman",
		AuthorURI:     "https://example.com/scoop",
		AuthorEmail:   "scoopidoo@example.com",
		AuthorTwitter: "",
	}
}

func ReadSiteMetadata(fsys writablefs.FS) (*SiteMetadata, error) {
	sm := NewSiteMetadata()

	data, err := fs.ReadFile(fsys, SettingsPath)
	if err != nil {
		return nil, fmt.Errorf("ReadSiteMetadata: %w", err)
	}

	UnmarshalMetadata(data, &sm)

	// normalize root path to always include leading & trailing slashes
	trimmed := strings.Trim(sm.Root, "/")
	if trimmed == "" {
		sm.Root = "/"
	} else {
		sm.Root = fmt.Sprintf("/%s/", trimmed)
	}

	// make sure AuthorTwitter starts with an "@"
	if len(sm.AuthorTwitter) > 0 && sm.AuthorTwitter[0] != '@' {
		sm.AuthorTwitter = "@" + sm.AuthorTwitter
	}

	// trim leading "/" because DefaultThumb will be prepended with web root
	// path when used.
	sm.DefaultThumb = strings.TrimPrefix(sm.DefaultThumb, "/")

	return &sm, nil
}

var timeFormats []string = []string{
	"2006-01-02",
	"2006-01-02 15:04",
	"2006-01-02 15:04:05",
}

// Similar API to json.Unmarshal but supports neither struct tags nor nesting.
func UnmarshalMetadata(data []byte, dest any) *errs.UserErr {
	m := metaTextToMap(data)

	s := reflect.ValueOf(dest).Elem()
	sType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fieldName := sType.Field(i).Name
		val, ok := m[fieldName]
		if ok {
			switch f.Type().String() {
			case "string":
				s.Field(i).SetString(val)

			case "int":
				intVal, err := strconv.Atoi(val)
				if err != nil {
					return &errs.UserErr{
						Field: fieldName,
						Msg:   fmt.Sprintf(`invalid int: "%s"`, err),
					}
				}
				s.Field(i).Set(reflect.ValueOf(intVal))

			case "bool":
				if val != "true" && val != "false" {
					return &errs.UserErr{
						Field: fieldName,
						Msg: fmt.Sprintf(
							`invalid boolean: expected true/false, got "%s"`,
							val,
						),
					}
				}
				s.Field(i).SetBool(val == "true")

			case "time.Time":
				var tVal time.Time
				var err error
				for _, f := range timeFormats {
					tVal, err = time.ParseInLocation(f, val, time.Local)
					if err == nil {
						break
					}
				}

				tVal = tVal.Local()
				if err != nil {
					return &errs.UserErr{
						Field: fieldName,
						Msg: fmt.Sprintf(
							`invalid date: expected YYYY-MM-DD[ HH:MM[:SS]], got "%s"`, val,
						),
					}
				}
				s.Field(i).Set(reflect.ValueOf(tVal))

			case "[]string":
				parts := strings.Split(val, ",")
				trimmed := make([]string, len(parts))
				for i := 0; i < len(parts); i++ {
					trimmed[i] = strings.TrimSpace(parts[i])
				}
				s.Field(i).Set(reflect.ValueOf(trimmed))

			case "main.PageType":
				pt, err := ParsePageType(val)
				if err != nil {
					return &errs.UserErr{
						Field: fieldName,
						Msg:   err.Error(),
					}
				}
				s.Field(i).Set(reflect.ValueOf(pt))

			case "template.HTML":
				html := template.HTML(val)
				s.Field(i).Set(reflect.ValueOf(html))

			default:
				panic(fmt.Sprintf(
					`unsupported metadata field type: "%s"`,
					f.Type().String(),
				))
			}
		}
	}
	return nil
}

func MarshalMetadata(v any) []byte {
	result := ""

	s := reflect.ValueOf(v).Elem()
	sType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		key := sType.Field(i).Name
		val := f.Interface()

		var repr string
		switch f.Type().String() {
		case "[]string":
			repr = strings.Join(val.([]string), ", ")
		default:
			repr = fmt.Sprintf("%v", val)
		}

		result += fmt.Sprintf("%s: %s\n", key, repr)
	}

	return []byte(result)
}

func metaTextToMap(s []byte) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(s)), "\n")
	for i, l := range lines {
		if len(l) == 0 || l[0] == '#' {
			continue
		}
		key, val, ok := strings.Cut(l, ":")
		if !ok {
			fmt.Printf("Metadata: invalid line %d: '%s'\n", i+1, l)
			continue
		}
		// The trimming will also clean up the stray CR in
		// Windows-style line breaks.
		result[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return result
}

var frontMatterSep = []byte("---")

func SeparateMetadata(r io.Reader) (metadata []byte, body []byte) {
	s := bufio.NewScanner(r)
	readingFrontMatter := true
	var buffer []byte
	for s.Scan() {
		line := s.Bytes()

		if readingFrontMatter {
			line = bytes.TrimSpace(s.Bytes())

			if bytes.Equal(line, frontMatterSep) {
				metadata = buffer
				buffer = body
				readingFrontMatter = false
				continue
			}
		}

		buffer = append(buffer, line...)
		buffer = append(buffer, '\n')
	}

	body = buffer
	return metadata, body
}
