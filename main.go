package main

import (
	"bytes"
	"context"
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
	"sync"
	"time"

	"go.imnhan.com/s4g/djot"
	"go.imnhan.com/s4g/errs"
	"go.imnhan.com/s4g/livereload"
	"go.imnhan.com/s4g/writablefs"
)

const DjotExt = ".dj"
const FeedPath = "feed.xml"
const S4gDir = "_s4g"

var SettingsPath = S4gDir + "/settings.txt"
var RedirectsPath = S4gDir + "/redirects.txt"
var ManifestPath = S4gDir + "/manifest"
var ThemePath = S4gDir + "/theme"

func main() {
	invalidCommand := func() {
		fmt.Println("Usage: s4g new|serve [...]")
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

	var serveFolder, servePort, serveHost string
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	serveCmd.StringVar(&serveFolder, "f", ".", "Website's root folder")
	serveCmd.StringVar(&serveHost, "h", "127.0.0.1", "Local server host")
	serveCmd.StringVar(&servePort, "p", "3338", "Local server port")

	switch cmd {
	case "new":
		newCmd.Parse(args)
		handleNewCmd(newFolder)
	case "serve":
		serveCmd.Parse(args)
		handleServeCmd(serveFolder, serveHost+":"+servePort)
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

func handleServeCmd(folder, addr string) {
	djot.StartService()
	fmt.Println("Started djot.js service")

	absolutePath, err := filepath.Abs(folder)
	if err != nil {
		panic(err)
	}

	fsys := writablefs.WriteDirFS(absolutePath)
	site, err := ReadSiteMetadata(fsys)
	if err != nil {
		panic(err)
	}

	webRootUpdates := make(chan string)

	var wg sync.WaitGroup
	wg.Add(1)
	go func(webRoot string) {
		defer wg.Done()

		srv := runServer(fsys, webRoot, addr)

		for {
			newRoot := <-webRootUpdates
			if newRoot == webRoot {
				continue
			}
			fmt.Println("Root changed => restarting server")
			webRoot = newRoot
			err := srv.Shutdown(context.TODO())
			if err != nil {
				panic(err)
			}
			srv = runServer(fsys, webRoot, addr)
		}
	}(site.Root)

	// TODO: only rebuild necessary bits instead of regenerating
	// the whole thing. To do that I'll probably need to:
	// - Devise some sort of dependency graph
	// - Filter out relevant FS events: this seems daunting considering the
	// differences between OSes and applications (e.g. vim writes to temp file
	// then renames), and fsnotify's inability to tell if the event came from a
	// directory.
	closeWatcher := WatchLocalFS(fsys, func() {
		fmt.Println("Change detected. Regenerating...")
		newSite, err := regenerate(fsys)
		livereload.SetError(err)
		if err == nil {
			webRootUpdates <- newSite.Root
		}
	})
	defer closeWatcher()

	_, err = regenerate(fsys)
	livereload.SetError(err)

	wg.Wait()
}

// Non-blocking. Returns srv handle to allow calling Shutdown() later.
func runServer(fsys writablefs.FS, webRoot, addr string) *http.Server {
	fmt.Printf("Serving local website at %s%s\n", addr, webRoot)
	mux := http.NewServeMux()
	mux.Handle(
		webRoot,
		livereload.Middleware(
			mux,
			webRoot,
			fsys,
			http.StripPrefix(webRoot, http.FileServer(http.FS(fsys))),
		),
	)

	if webRoot != "/" {
		mux.Handle("/", http.RedirectHandler(webRoot, http.StatusTemporaryRedirect))
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		srv.ListenAndServe()
	}()

	return srv
}

func regenerate(fsys writablefs.FS) (site *SiteMetadata, err error) {
	defer timer("Took %s")()

	site, err = ReadSiteMetadata(fsys)
	if err != nil {
		livereload.SetError(err)
		return nil, err
	}

	articles, err := findArticles(fsys, site)
	if err != nil {
		return nil, err
	}

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
			return nil, &errs.UserErr{
				File:  SettingsPath,
				Field: "NavbarLinks",
				Msg:   fmt.Sprintf(`"%s" does not exist`, link),
			}
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
		err := a.WriteHtmlFile(site, articlesInNav, articlesInFeed, startYear)
		if err != nil {
			return nil, fmt.Errorf("Article %s: %w", a.Path, err)
		}
		generatedFiles[a.OutputPath] = true
	}
	fmt.Printf("Processed %d articles\n", len(articles))

	if len(articlesInFeed) > 0 {
		fsys.WriteFile(
			FeedPath,
			generateFeed(site, articlesInFeed, site.Root+FeedPath),
		)
		generatedFiles[FeedPath] = true
		fmt.Println("Generated", FeedPath)
	}

	redirects, uerr := generateRedirects(fsys, RedirectsPath, site.Root)
	if uerr != nil {
		return nil, fmt.Errorf("generate redirects: %w", uerr)
	}
	for _, p := range redirects {
		generatedFiles[p] = true
	}
	fmt.Printf("Generated %d redirects\n", len(redirects))

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
	WebPath        string
	TemplatePaths  []string
	OpenGraphImage string
}

func (a *Article) ComputeDerivedFields(addr, root string) {
	a.computeWebPath(root)
	a.computeTemplatePaths()

	if a.Thumb != "" {
		a.OpenGraphImage = addr + root + filepath.Dir(a.Path) + "/" + a.Thumb
	}
}

func (a *Article) computeWebPath(root string) {
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

func (a *Article) computeTemplatePaths() {
	var templates []string
	switch a.PageType {
	case PTPost:
		templates = []string{
			"$base.tmpl",
			"$includes.tmpl",
			"$post.tmpl",
		}
	case PTHome:
		templates = []string{
			"$base.tmpl",
			"$includes.tmpl",
			"$home.tmpl",
		}
	case PTSeriesIndex:
		templates = []string{
			"$base.tmpl",
			"$includes.tmpl",
			"$series-index.tmpl",
		}
	case PTCustom:
		templates = a.Templates
	default:
		panic(fmt.Sprintf("Invalid PageType: %v", a.PageType))
	}

	paths := make([]string, len(templates))
	for i := 0; i < len(paths); i++ {
		p := templates[i]
		if strings.HasPrefix(p, "$") {
			paths[i] = ThemePath + "/" + strings.TrimPrefix(p, "$")
		} else {
			paths[i] = filepath.Join(filepath.Dir(a.Path), p)
		}
	}

	a.TemplatePaths = paths
}

func (a *Article) WriteHtmlFile(
	site *SiteMetadata,
	articlesInNav []Article,
	articlesInFeed []Article,
	startYear int,
) error {
	contentHtml := djot.ToHtml(a.DjotBody)

	tmpl, err := template.ParseFS(a.Fs, a.TemplatePaths...)
	// TODO: should probably reuse the template object for common cases
	if err != nil {
		return fmt.Errorf(
			"Failed to parse templates (%v): %w", a.TemplatePaths, err,
		)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, struct {
		Site           *SiteMetadata
		Content        template.HTML
		Title          string
		Post           *Article
		ArticlesInNav  []Article
		ArticlesInFeed []Article
		Feed           string
		Now            time.Time
		StartYear      int
		ThemePath      string
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
		ThemePath:      site.Root + ThemePath,
	})
	if err != nil {
		return fmt.Errorf("Failed to execute templates (%v): %w", a.TemplatePaths, err)
	}
	fullHtml := buf.Bytes()

	// Now write into an html with the same name as the original djot file
	err = a.Fs.WriteFile(a.OutputPath, fullHtml)
	if err != nil {
		return fmt.Errorf("Failed to write to %s: %w", a.OutputPath, err)
	}

	return nil
}

func findArticles(fsys writablefs.FS, site *SiteMetadata) (map[string]Article, error) {
	result := make(map[string]Article)

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
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
			PageType:   PTPost,
			ShowInFeed: true,
		}
		userErr := UnmarshalMetadata(metaText, &meta)
		if userErr != nil {
			userErr.File = path
			return fmt.Errorf("findArticles failed to unmarshall metadata: %w", userErr)
		}

		if meta.PageType != PTCustom && len(meta.Templates) > 0 {
			return &errs.UserErr{
				File:  path,
				Field: "PageType",
				Msg:   `you must set "PageType: custom" in order to use custom Templates`,
			}
		}

		if meta.PageType == PTCustom && len(meta.Templates) == 0 {
			return &errs.UserErr{
				File:  path,
				Field: "Templates",
				Msg:   `custom PageType requires a non-empty Templates list`,
			}
		}

		article := Article{
			Fs:              fsys,
			Path:            path,
			OutputPath:      strings.TrimSuffix(path, DjotExt) + ".html",
			DjotBody:        bodyText,
			ArticleMetadata: meta,
		}
		article.ComputeDerivedFields(site.Address, site.Root)
		result[article.Path] = article
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
