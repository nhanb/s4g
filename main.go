package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"go.imnhan.com/webmaker2000/djot"
	"go.imnhan.com/webmaker2000/livereload"
	"go.imnhan.com/webmaker2000/writablefs"
)

const DjotExt = ".dj"
const SiteExt = ".wbmkr2k"
const SiteFileName = "website" + SiteExt
const FeedPath = "feed.xml"

func main() {
	var port, folder, new string
	flag.StringVar(&new, "new", "", "Path for new site to make")
	flag.StringVar(&port, "port", "3338", "Port for local preview server")
	flag.StringVar(&folder, "folder", "www", "Web folder")
	flag.Parse()

	if new != "" {
		fmt.Println("Making new site at", new)
		err := makeSite(new, newSiteMetadata())
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	djot.StartService()
	fmt.Println("Started djot.js service")

	absolutePath, err := filepath.Abs(folder)
	if err != nil {
		panic(err)
	}

	fsys := writablefs.WriteDirFS(absolutePath)

	regenerate(fsys)

	// TODO: only rebuild necessary bits instead of regenerating
	// the whole thing. To do that I'll probably need to:
	// - Devise some sort of dependency graph
	// - Filter out relevant FS events: this seems daunting considering the
	// differences between OSes and applications (e.g. vim writes to temp file
	// then renames), and fsnotify's inability to tell if the event came from a
	// directory.
	closeWatcher := WatchLocalFS(fsys, func() {
		fmt.Println("Change detected. Regenerating...")
		regenerate(fsys)
		livereload.Trigger()
	})
	defer closeWatcher()

	println("Serving local website at http://localhost:" + port)
	http.Handle("/", livereload.Middleware(fsys, http.FileServer(http.FS(fsys))))
	err = http.ListenAndServe("127.0.0.1:"+port, nil)
	if err != nil {
		panic(err)
	}
}

func regenerate(fsys writablefs.FS) {
	defer timer("Took %s")()

	site := readSiteMetadata(fsys)
	articles := findArticles(fsys)

	if len(articles) == 0 {
		fmt.Println("No articles found.")
		fsys.RemoveAll("index.html")
		fsys.RemoveAll(FeedPath)
		return
	}

	generatedFiles := make(map[string]bool)

	// Sort articles, newest first
	sort.Slice(articles, func(i int, j int) bool {
		return articles[i].PostedAt.Compare(articles[j].PostedAt) > 0
	})

	var startYear int

	var articlesInNav, articlesInFeed []Article
	for _, a := range articles {
		if a.ShowInNav {
			articlesInNav = append(articlesInNav, a)
		}
		if a.ShowInFeed {
			articlesInFeed = append(articlesInFeed, a)
		}
		if !a.PostedAt.IsZero() {
			startYear = a.PostedAt.Year()
		}
	}

	if startYear == 0 {
		startYear = time.Now().Year()
	}

	for _, a := range articles {
		fmt.Println(">", a.Path, "-", a.Title)
		a.WriteHtmlFile(&site, articlesInNav, startYear)
		generatedFiles[a.WebPath] = true
	}
	fmt.Printf("Processed %d articles\n", len(articles))

	if site.GenerateHome {
		WriteHomePage(fsys, site, articlesInFeed, articlesInNav, startYear)
		generatedFiles["index.html"] = true
		fmt.Println("Generated index.html")
	}

	fsys.WriteFile(
		FeedPath,
		generateFeed(site, articlesInFeed, site.HomePath+FeedPath),
	)
	generatedFiles[FeedPath] = true
	fmt.Println("Generated", FeedPath)

	DeleteOldGeneratedFiles(fsys, generatedFiles)
	WriteManifest(fsys, generatedFiles)
}

type SiteMetadata struct {
	Address      string
	Name         string
	Tagline      string
	HomePath     string
	ShowFooter   bool
	GenerateHome bool
	Author       struct {
		Name  string
		URI   string
		Email string
	}
}

func newSiteMetadata() SiteMetadata {
	return SiteMetadata{
		HomePath:     "/",
		ShowFooter:   true,
		GenerateHome: true,
	}
}

func readSiteMetadata(fsys writablefs.FS) SiteMetadata {
	sm := newSiteMetadata()
	_, err := toml.DecodeFS(fsys, SiteFileName, &sm)
	if err != nil {
		panic(err)
	}
	return sm
}

type Article struct {
	Fs       writablefs.FS
	Path     string
	WebPath  string
	DjotBody string
	ArticleMetadata
}

type ArticleMetadata struct {
	Title      string
	IsDraft    bool
	PostedAt   time.Time
	Templates  []string
	ShowInFeed bool
	ShowInNav  bool
}

func (a *Article) WriteHtmlFile(
	site *SiteMetadata,
	articlesInNav []Article,
	startYear int,
) {
	// First generate the main content in html
	contentHtml := djot.ToHtml(a.DjotBody)

	// Then insert that content into the main template
	var buf bytes.Buffer
	// TODO: should probably reuse the template object for common cases
	tmpl := template.Must(template.ParseFS(a.Fs, a.Templates...))
	err := tmpl.Execute(&buf, struct {
		Site          *SiteMetadata
		Content       template.HTML
		Title         string
		Post          *Article
		ArticlesInNav []Article
		Feed          string
		Now           time.Time
		StartYear     int
	}{
		Site:          site,
		Content:       template.HTML(contentHtml),
		Title:         fmt.Sprintf("%s | %s", a.Title, site.Name),
		Post:          a,
		ArticlesInNav: articlesInNav,
		Feed:          site.HomePath + FeedPath,
		Now:           time.Now(),
		StartYear:     startYear,
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
	fsys writablefs.FS,
	site SiteMetadata,
	articlesInFeed, articlesInNav []Article,
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
		Site           *SiteMetadata
		Title          string
		ArticlesInFeed []Article
		ArticlesInNav  []Article
		Feed           string
		Now            time.Time
		StartYear      int
	}{
		Site:           &site,
		Title:          fmt.Sprintf("%s - %s", site.Name, site.Tagline),
		ArticlesInFeed: articlesInFeed,
		ArticlesInNav:  articlesInNav,
		Feed:           site.HomePath + FeedPath,
		Now:            time.Now(),
		StartYear:      startYear,
	})
	if err != nil {
		fmt.Println("Error in WriteHtmlFile:", err)
		return
	}
	fsys.WriteFile("index.html", buf.Bytes())
}

func findArticles(fsys writablefs.FS) (result []Article) {

	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || !strings.HasSuffix(d.Name(), DjotExt) {
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

		meta := ArticleMetadata{
			Templates:  []string{"_theme/base.tmpl", "_theme/post.tmpl"},
			ShowInFeed: true,
			ShowInNav:  false,
		}
		_, err = toml.Decode(metaText, &meta)
		if err != nil {
			fmt.Printf("FIXME: Malformed article metadata in %s: %s\n", path, err)
			return nil
		}

		article := Article{
			Fs:              fsys,
			Path:            path,
			WebPath:         strings.TrimSuffix(path, DjotExt) + ".html",
			DjotBody:        bodyText,
			ArticleMetadata: meta,
		}
		result = append(result, article)
		return nil
	})
	return
}
