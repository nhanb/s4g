package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.imnhan.com/s4g/writablefs"
)

var WatchedExts = []string{DjotExt, ".tmpl", ".txt"}

const debounceInterval = 500 * time.Millisecond

// Watches for relevant changes in FS, debounces by debounceInterval,
// then executes callback.
// Returns cleanup function.
func WatchLocalFS(fsys writablefs.FS, callback func()) (Close func() error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	fsysPath := fsys.Path()

	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() || (shouldIgnore(path) && path != ".") {
			return nil
		}

		fullPath := filepath.Join(fsysPath, path)

		err = watcher.Add(fullPath)
		if err != nil {
			panic(err)
		}

		return nil
	})

	//printWatchList(watcher)

	// Start listening for events.
	events := make(chan struct{})
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				//relPath, err := filepath.Rel(fsysPath, event.Name)
				//if err != nil {
				//panic(err)
				//}
				//fmt.Println("EVENT:", event.Op, relPath)

				if shouldIgnore(event.Name) {
					break
				}

				// Avoid infinite loop
				if event.Has(fsnotify.Write) &&
					!contains(WatchedExts, filepath.Ext(event.Name)) {
					break
				}

				// Dynamically watch new/renamed folders
				if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
					stat, err := os.Stat(event.Name)
					if err == nil && stat.IsDir() {
						watcher.Add(event.Name)
					}
				}

				events <- struct{}{}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("error:", err)
			}
		}
	}()

	// Debounce
	go func() {
		timer := time.NewTimer(debounceInterval)
		<-timer.C // drain once so callback isn't executed on startup
		for {
			select {
			case <-events:
				timer.Reset(debounceInterval)
			case <-timer.C:
				callback()
			}
		}
	}()

	return watcher.Close
}

func printWatchList(w *fsnotify.Watcher) {
	fmt.Println("WatchList:")
	for _, path := range w.WatchList() {
		fmt.Println("  " + path)
	}
}

// Ignore swap and dot files/dirs, which are typically editor
// temp files or supporting data like .git.
func shouldIgnore(path string) bool {
	fname := filepath.Base(path)
	return fname[0] == '.' ||
		fname == ManifestPath ||
		strings.HasSuffix(fname, ".swp")
}
