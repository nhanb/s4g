package writablefs

import (
	"io/fs"
	"os"
	"path/filepath"
)

type FS interface {
	fs.FS
	WriteFile(path string, content []byte) error
	RemoveAll(path string) error
	MkdirAll(path string) error
	Path() string
}

// Like os.DirFS but is writable
func WriteDirFS(path string) FS {
	return writeDirFS(path)
}

type writeDirFS string

func (w writeDirFS) Open(name string) (fs.File, error) {
	return os.DirFS(string(w)).Open(name)
}
func (w writeDirFS) RemoveAll(path string) error {
	fullPath := filepath.Join(string(w), path)
	return os.RemoveAll(fullPath)
}

func (w writeDirFS) WriteFile(path string, content []byte) error {
	fullPath := filepath.Join(string(w), path)
	return os.WriteFile(fullPath, content, 0644)
}

func (w writeDirFS) Path() string {
	return string(w)
}

func (w writeDirFS) MkdirAll(path string) error {
	fullPath := filepath.Join(string(w), path)
	return os.MkdirAll(fullPath, 0755)
}
