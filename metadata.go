package main

import (
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"time"

	"go.imnhan.com/webmaker2000/writablefs"
)

type SiteMetadata struct {
	Address      string
	Name         string
	Tagline      string
	HomePath     string
	ShowFooter   bool
	GenerateHome bool
	AuthorName   string
	AuthorURI    string
	AuthorEmail  string
}

type ArticleMetadata struct {
	Title      string
	IsDraft    bool
	PostedAt   time.Time
	Templates  []string
	ShowInFeed bool
	ShowInNav  bool
}

func NewSiteMetadata() SiteMetadata {
	return SiteMetadata{
		HomePath:     "/",
		ShowFooter:   true,
		GenerateHome: true,
	}
}

func ReadSiteMetadata(fsys writablefs.FS) SiteMetadata {
	sm := NewSiteMetadata()

	data, err := fs.ReadFile(fsys, SiteFileName)
	if err != nil {
		panic(err)
	}

	UnmarshalMetadata(data, &sm)
	return sm
}

// Similar API to json.Unmarshal but supports neither struct tags nor nesting.
func UnmarshalMetadata(data []byte, dest any) error {
	m := metaTextToMap(data)

	s := reflect.ValueOf(dest).Elem()
	sType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		val, ok := m[sType.Field(i).Name]
		if ok {
			switch f.Type().String() {
			case "string":
				s.Field(i).SetString(val)

			case "bool":
				if val != "true" && val != "false" {
					return fmt.Errorf(
						"invalid boolean: expected true/false, got %s", val,
					)
				}
				s.Field(i).SetBool(val == "true")

			case "time.Time":
				tVal, err := time.ParseInLocation("2006-01-02", val, time.Local)
				tVal = tVal.Local()
				if err != nil {
					return fmt.Errorf(
						"invalid date: expected YYYY-MM-DD, got %s", val,
					)
				}
				s.Field(i).Set(reflect.ValueOf(tVal))

			default:
				panic(fmt.Sprintf(
					"unsupported metadata field type: %s",
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
		result += fmt.Sprintf("%s: %v\n", key, val)
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
