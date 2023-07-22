package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"strings"

	"go.imnhan.com/webmaker2000/errs"
	"go.imnhan.com/webmaker2000/writablefs"
)

// Returns list of generated files
func generateRedirects(fsys writablefs.FS, path string, root string) ([]string, *errs.UserErr) {
	f, err := fsys.Open(path)
	if err != nil {
		panic(err)
	}

	var sources, dests []string

	s := bufio.NewScanner(f)
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		src, dest, found := strings.Cut(line, "->")
		if !found {
			return nil, &errs.UserErr{
				File: path,
				Line: lineNo,
				Msg:  fmt.Sprintf(`Expected "src -> dest", found "%s"`, line),
			}
		}

		src = strings.TrimPrefix(strings.TrimSpace(src), "/")
		dest = strings.TrimPrefix(strings.TrimSpace(dest), "/")

		if strings.HasSuffix(src, "/") {
			return nil, &errs.UserErr{
				File: path,
				Line: lineNo,
				Msg:  fmt.Sprintf(`Source must not end with a "/" (found "%s")`, line),
			}
		}

		srcStat, err := fs.Stat(fsys, src)
		if err == nil {
			if srcStat.IsDir() {
				return nil, &errs.UserErr{
					File: path,
					Line: lineNo,
					Msg:  fmt.Sprintf(`Source must not be a folder (found "%s")`, line),
				}
			}
		}

		sources = append(sources, src)
		dests = append(dests, dest)
	}

	for i, src := range sources {
		srcDir := filepath.Dir(src)
		err := fsys.MkdirAll(srcDir)
		if err != nil {
			panic(err)
		}

		var srcBuf bytes.Buffer
		err = srcTmpl.Execute(&srcBuf, root+dests[i])
		if err != nil {
			panic(err)
		}

		err = fsys.WriteFile(src, srcBuf.Bytes())
		if err != nil {
			panic(err)
		}

		fmt.Printf("Redirect: %s -> %s\n", src, dests[i])
	}

	return sources, nil
}

var srcTmpl = template.Must(template.New("src").Parse(`<!DOCTYPE html>
<html lang="en">
  <head>
    <title>Redirecting to {{.}}</title>
    <meta http-equiv="Refresh" content="0; URL={{.}}" />
  </head>
  <body>
    The page you're looking for has been moved to <a href="{{.}}">{{.}}</a>.
  </body>
</html>
`))
