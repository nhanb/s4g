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
	serveCmd.StringVar(&serveFolder, "f", "docs", "Folder for existing website")
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

	site := regenerate(fsys)

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

	println("Serving local website at http://localhost:" + port + site.Root)
	http.Handle(
		site.Root,
		livereload.Middleware(
			site.Root,
			fsys,
			http.StripPrefix(site.Root, http.FileServer(http.FS(fsys))),
		),
	)

	if site.Root != "/" {
		http.Handle("/", http.RedirectHandler(site.Root, http.StatusTemporaryRedirect))
	}

	err = http.ListenAndServe("127.0.0.1:"+port, nil)
	if err != nil {
		panic(err)
	}
}

func regenerate(fsys writablefs.FS) (site SiteMetadata) {
	defer timer("Took %s")()

	site = ReadSiteMetadata(fsys)
	articles := findArticles(fsys, site)

	if len(articles) == 0 {
		fmt.Println("No articles found.")
		fsys.RemoveAll(FeedPath)
		return
	}

	generatedFiles := make(map[string]bool)

	var articlesInNav []Article
	for _, link := range site.NavbarLinks {
		a, ok := articles[link]
		if !ok {
			fmt.Printf("NavbarLinks: %s not found\n", link)
			continue
		}
		articlesInNav = append(articlesInNav, a)
	}

	var articlesInFeed []Article
	startYear := time.Now().Year()
	for _, a := range articles {
		if a.ShowInFeed {
			articlesInFeed = append(articlesInFeed, a)
		}
		if !a.PostedAt.IsZero() && a.PostedAt.Year() < startYear {
			startYear = a.PostedAt.Year()
		}
	}

	// Sort articles in feed, newest first
	sort.Slice(articlesInFeed, func(i int, j int) bool {
		return articlesInFeed[i].PostedAt.Compare(articlesInFeed[j].PostedAt) > 0
	})

	for _, a := range articles {
		fmt.Println(">", a.Path, "-", a.Title)
		a.WriteHtmlFile(&site, articlesInNav, articlesInFeed, startYear)
		generatedFiles[a.OutputPath] = true
	}
	fmt.Printf("Processed %d articles\n", len(articles))

	fsys.WriteFile(
		FeedPath,
		generateFeed(site, articlesInFeed, site.Root+FeedPath),
	)
	generatedFiles[FeedPath] = true
	fmt.Println("Generated", FeedPath)

	DeleteOldGeneratedFiles(fsys, generatedFiles)
	WriteManifest(fsys, generatedFiles)

	return
}

type Article struct {
	Fs         writablefs.FS
	Path       string
	OutputPath string
	DjotBody   []byte
	ArticleMetadata
	WebPath       string
	templatePaths []string
}

func (a *Article) ComputeWebPath(root string) {
	webPath := root + a.OutputPath
	if strings.HasSuffix(webPath, "/index.html") {
		webPath = strings.TrimSuffix(webPath, "index.html")
	}

	parts := strings.Split(webPath, "/")
	escaped := make([]string, len(parts))
	for i := 0; i < len(parts); i++ {
		escaped[i] = url.PathEscape(parts[i])
	}

	a.WebPath = strings.Join(escaped, "/")
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
	articlesInFeed []Article,
	startYear int,
) {
	// First generate the main content in html
	contentHtml := djot.ToHtml(a.DjotBody)

	// Then insert that content into the main template
	var buf bytes.Buffer
	// TODO: should probably reuse the template object for common cases
	tmpl := template.Must(template.ParseFS(a.Fs, a.TemplatePaths()...))
	err := tmpl.Execute(&buf, struct {
		Site           *SiteMetadata
		Content        template.HTML
		Title          string
		Post           *Article
		ArticlesInNav  []Article
		ArticlesInFeed []Article
		Feed           string
		Now            time.Time
		StartYear      int
	}{
		Site:           site,
		Content:        template.HTML(contentHtml),
		Title:          a.Title,
		Post:           a,
		ArticlesInNav:  articlesInNav,
		ArticlesInFeed: articlesInFeed,
		Feed:           site.Root + FeedPath,
		Now:            time.Now(),
		StartYear:      startYear,
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

func findArticles(fsys writablefs.FS, site SiteMetadata) map[string]Article {
	result := make(map[string]Article)

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
		article.ComputeWebPath(site.Root)
		result[article.Path] = article
		return nil
	})
	return result
}
