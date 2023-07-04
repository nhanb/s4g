package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"go.imnhan.com/webmaker2000/djot"
)

const DJOT_EXT = ".dj"
const FEED_PATH = "feed.xml"

func main() {
	var port, folder string
	flag.StringVar(&port, "port", "3338", "Port for local preview server")
	flag.StringVar(&folder, "folder", "www", "Web folder")
	flag.Parse()

	absolutePath, err := filepath.Abs(folder)
	if err != nil {
		panic(err)
	}

	fsys := WriteDirFS(absolutePath)
	site := readSiteMetadata(fsys)

	posts, pages := findArticles(fsys)

	// Sort posts, newest first
	sort.Slice(posts, func(i int, j int) bool {
		return posts[i].Meta.PostedAt.Compare(posts[j].Meta.PostedAt) > 0
	})

	startYear := posts[len(posts)-1].Meta.PostedAt.Year()

	fmt.Printf("Found %d posts, %d pages:\n", len(posts), len(pages))
	for _, a := range posts {
		fmt.Println(">", a.Path, "-", a.Meta.Title)
		a.WriteHtmlFile(&site, pages, startYear)
	}
	for _, a := range pages {
		fmt.Println(">", a.Path, "-", a.Meta.Title)
		a.WriteHtmlFile(&site, pages, startYear)
	}

	WriteHomePage(fsys, site, posts, pages, startYear)

	fsys.WriteFile(FEED_PATH, generateFeed(site, posts, site.HomePath+FEED_PATH))

	println("Serving local website at http://localhost:" + port)
	http.Handle("/", http.FileServer(http.FS(fsys)))
	err = http.ListenAndServe("127.0.0.1:"+port, nil)
	if err != nil {
		panic(err)
	}
}

type SiteMetadata struct {
	Address       string
	Name          string
	Tagline       string
	HomePath      string
	DisableFooter bool
	Author        struct {
		Name  string
		URI   string
		Email string
	}
}

func readSiteMetadata(fsys WritableFS) (sm SiteMetadata) {
	_, err := toml.DecodeFS(fsys, "website.toml", &sm)
	if err != nil {
		panic(err)
	}
	if sm.HomePath == "" {
		sm.HomePath = "/"
	}
	return sm
}

type Article struct {
	Fs       WritableFS
	Path     string
	WebPath  string
	DjotBody string
	Meta     ArticleMetadata
}

type ArticleMetadata struct {
	Title    string
	IsPage   bool
	IsDraft  bool
	PostedAt time.Time
}

func (a *Article) WriteHtmlFile(site *SiteMetadata, pages []Article, startYear int) {
	// First generate the main content in html
	contentHtml := djot.ToHtml(a.DjotBody)

	// Then insert that content into the main template
	var buf bytes.Buffer
	tmpl := template.Must(
		template.ParseFS(
			a.Fs,
			"_theme/base.tmpl",
			"_theme/post.tmpl",
		),
	)
	err := tmpl.Execute(&buf, struct {
		Site      *SiteMetadata
		Content   template.HTML
		Title     string
		Post      *Article
		Pages     []Article
		Feed      string
		Now       time.Time
		StartYear int
	}{
		Site:      site,
		Content:   template.HTML(contentHtml),
		Title:     fmt.Sprintf("%s | %s", a.Meta.Title, site.Name),
		Post:      a,
		Pages:     pages,
		Feed:      site.HomePath + FEED_PATH,
		Now:       time.Now(),
		StartYear: startYear,
	})
	if err != nil {
		fmt.Println("Error in WriteHtmlFile:", err)
		return
	}
	fullHtml := buf.Bytes()

	// Now write into an html with the same name as the original djot file
	err = a.Fs.WriteFile(a.WebPath, fullHtml)
	if err != nil {
		panic(err)
	}
}

func WriteHomePage(
	fsys WritableFS,
	site SiteMetadata,
	posts, pages []Article,
	startYear int,
) {
	var buf bytes.Buffer
	tmpl := template.Must(
		template.ParseFS(
			fsys,
			"_theme/base.tmpl",
			"_theme/home.tmpl",
		),
	)
	err := tmpl.Execute(&buf, struct {
		Site      *SiteMetadata
		Title     string
		Posts     []Article
		Pages     []Article
		Feed      string
		Now       time.Time
		StartYear int
	}{
		Site:      &site,
		Title:     fmt.Sprintf("%s - %s", site.Name, site.Tagline),
		Posts:     posts,
		Pages:     pages,
		Feed:      site.HomePath + FEED_PATH,
		Now:       time.Now(),
		StartYear: startYear,
	})
	if err != nil {
		fmt.Println("Error in WriteHtmlFile:", err)
		return
	}
	fsys.WriteFile("index.html", buf.Bytes())
}

func findArticles(fsys WritableFS) (posts, pages []Article) {

	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || !strings.HasSuffix(d.Name(), DJOT_EXT) {
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
			Fs:       fsys,
			Path:     path,
			WebPath:  strings.TrimSuffix(path, DJOT_EXT) + ".html",
			DjotBody: bodyText,
			Meta:     meta,
		}
		if article.Meta.IsPage {
			pages = append(pages, article)
		} else {
			posts = append(posts, article)
		}
		return nil
	})
	return
}
