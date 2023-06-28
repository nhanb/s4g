package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

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

	findPosts(fsys)

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
	Title    string
	DjotBody string
}
type Post Article
type Page Article

func findPosts(fsys fs.FS) (posts []Post) {
	var paths []string
	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if d.Name() == "post.toml" {
			paths = append(paths, filepath.Dir(path))
		}
		return nil
	})
	return posts
}
