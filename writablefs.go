package main

import (
	"io/fs"
	"os"
	"path/filepath"
)

type WritableFS interface {
	fs.FS
	WriteFile(path string, content string) error
}

// Like os.DirFS but is writable
func WriteDirFS(path string) WritableFS {
	return writeDirFS(path)
}

type writeDirFS string

func (w writeDirFS) Open(name string) (fs.File, error) {
	return os.DirFS(string(w)).Open(name)
}

func (w writeDirFS) WriteFile(path string, content string) error {
	fullPath := filepath.Join(string(w), path)
	return os.WriteFile(fullPath, []byte(content), 0644)
}
