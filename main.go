package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

func main() {
	var port, folder string
	flag.StringVar(&port, "port", "3338", "Port for local preview server")
	flag.StringVar(&folder, "folder", "www", "Web folder")
	flag.Parse()

	absolutePath, err := filepath.Abs(folder)
	if err != nil {
		panic(err)
	}
	fsys := os.DirFS(absolutePath)

	meta := readSiteMetadata(fsys)
	fmt.Println("Found site:", meta)

	articles := findArticles(fsys)
	fmt.Printf("Found %d articles:\n", len(articles))
	for _, a := range articles {
		fmt.Println(">", a.Path, "-", a.Meta.Title)
	}

	println("Serving local website at http://localhost:" + port)
	http.Handle("/", http.FileServer(http.FS(fsys)))
	err = http.ListenAndServe("127.0.0.1:"+port, nil)
	if err != nil {
		panic(err)
	}
}

type SiteMetadata struct {
	Name    string
	Tagline string
}

func readSiteMetadata(fsys fs.FS) (sm SiteMetadata) {
	_, err := toml.DecodeFS(fsys, "website.toml", &sm)
	if err != nil {
		panic(err)
	}
	return sm
}

type Article struct {
	Path     string
	DjotBody string
	Meta     ArticleMetadata
}

type ArticleMetadata struct {
	Title  string
	IsPage bool
}

func findArticles(fsys fs.FS) (articles []Article) {

	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".dj") {
			return nil
		}

		fileContent, err := fs.ReadFile(fsys, path)
		if err != nil {
			panic(err)
		}

		parts := strings.SplitN(string(fileContent), "+++", 3)
		if !(len(parts) == 3 && parts[0] == "") {
			fmt.Printf("FIXME: Missing metadata in %s - Skipped.\n", path)
			return nil
		}
		metaText := strings.TrimSpace(parts[1])
		bodyText := strings.TrimSpace(parts[2])

		var meta ArticleMetadata
		_, err = toml.Decode(metaText, &meta)
		if err != nil {
			fmt.Printf("FIXME: Malformed article metadata in %s: %s", path, err)
			return nil
		}

		article := Article{
			Path:     path,
			DjotBody: bodyText,
			Meta:     meta,
		}
		articles = append(articles, article)
		fmt.Printf("Found article %s - %s\n", article.Path, article.Meta.Title)
		return nil
	})
	return articles
}
