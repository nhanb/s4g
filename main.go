package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.imnhan.com/webmaker2000/djot"
	"go.imnhan.com/webmaker2000/livereload"
	"go.imnhan.com/webmaker2000/writablefs"
)

const DjotExt = ".dj"
const SiteExt = ".wbmkr2k"
const SiteFileName = "website" + SiteExt
const FeedPath = "feed.xml"

func main() {
	invalidCommand := func() {
		fmt.Println("Usage: webfolder2000 new|serve [...]")
		os.Exit(1)
	}

	// If no subcommand is given, default to "serve"
	var cmd string
	var args []string
	if len(os.Args) < 2 {
		cmd = "serve"
		args = os.Args[1:]
	} else {
		cmd = os.Args[1]
		args = os.Args[2:]
	}

	var newFolder string
	newCmd := flag.NewFlagSet("new", flag.ExitOnError)
	newCmd.StringVar(&newFolder, "f", "site1", "Folder for new website")

	var serveFolder, servePort string
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	serveCmd.StringVar(&serveFolder, "f", "www", "Folder for existing website")
	serveCmd.StringVar(&servePort, "p", "3338", "Port for local preview server")

	switch cmd {
	case "new":
		newCmd.Parse(args)
		handleNewCmd(newFolder)
	case "serve":
		serveCmd.Parse(args)
		handleServeCmd(serveFolder, servePort)
	default:
		invalidCommand()
	}
}

func handleNewCmd(folder string) {
	fmt.Println("Making new site at", folder)
	err := makeSite(folder, NewSiteMetadata())
	if err != nil {
		log.Fatal(err)
	}
}

func handleServeCmd(folder, port string) {
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

	site := ReadSiteMetadata(fsys)
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
		generatedFiles[a.OutputPath] = true
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

type Article struct {
	Fs         writablefs.FS
	Path       string
	OutputPath string
	DjotBody   []byte
	ArticleMetadata
	webPath       string
	templatePaths []string
}

func (a *Article) WebPath() string {
	if a.webPath != "" {
		return a.webPath
	}
	path := a.OutputPath
	if strings.HasSuffix(path, "/index.html") {
		path = strings.TrimSuffix(path, "index.html")
	}

	parts := strings.Split(path, "/")
	escaped := make([]string, len(parts))
	for i := 0; i < len(parts); i++ {
		escaped[i] = url.PathEscape(parts[i])
	}

	a.webPath = strings.Join(escaped, "/")
	return a.webPath
}

func (a *Article) TemplatePaths() []string {
	if len(a.templatePaths) > 0 {
		return a.templatePaths
	}
	paths := make([]string, len(a.Templates))
	for i := 0; i < len(paths); i++ {
		p := a.Templates[i]
		if strings.HasPrefix(p, "$") {
			paths[i] = strings.TrimPrefix(p, "$")
		} else {
			paths[i] = filepath.Join(filepath.Dir(a.Path), p)
		}
	}

	a.templatePaths = paths
	return paths
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
	tmpl := template.Must(template.ParseFS(a.Fs, a.TemplatePaths()...))
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
	err = a.Fs.WriteFile(a.OutputPath, fullHtml)
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
			"_theme/includes.tmpl",
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

		file, err := fsys.Open(path)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		metaText, bodyText := SeparateMetadata(file)
		if len(metaText) == 0 {
			fmt.Printf("FIXME: Metadata not found in %s\n", path)
			return nil
		}

		meta := ArticleMetadata{
			Templates: []string{
				"$_theme/base.tmpl",
				"$_theme/includes.tmpl",
				"$_theme/post.tmpl",
			},
			ShowInFeed: true,
			ShowInNav:  false,
		}
		err = UnmarshalMetadata(metaText, &meta)
		if err != nil {
			fmt.Printf("FIXME: Malformed article metadata in %s: %s\n", path, err)
			return nil
		}

		article := Article{
			Fs:              fsys,
			Path:            path,
			OutputPath:      strings.TrimSuffix(path, DjotExt) + ".html",
			DjotBody:        bodyText,
			ArticleMetadata: meta,
		}
		result = append(result, article)
		return nil
	})
	return
}
