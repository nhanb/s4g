package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

var WATCHED_EXTS = []string{DJOT_EXT, SITE_EXT, ".tmpl"}

const debounceInterval = 500 * time.Millisecond

// Watches for relevant changes in FS, debounces by debounceInterval,
// then executes callback.
// Returns cleanup function.
func WatchLocalFS(fsys WritableFS, callback func()) (Close func() error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			return nil
		}

		fullPath := filepath.Join(fsys.Path(), path)

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

				// Avoid infinite loop
				if event.Has(fsnotify.Write) &&
					!contains(WATCHED_EXTS, filepath.Ext(event.Name)) {
					break
				}

				//fmt.Println("EVENT:", event.Op, event.Name)

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

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
