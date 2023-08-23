package main

import (
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed theme
var defaultTheme embed.FS

func makeSite(path string, meta SiteMetadata) error {
	// Create web root dir
	err := os.MkdirAll(filepath.Join(path, S4gDir), 0755)
	if err != nil {
		return fmt.Errorf("make site: %w", err)
	}

	// Write site metadata file
	data := MarshalMetadata(&meta)
	err = os.WriteFile(filepath.Join(path, SettingsPath), data, 0664)
	if err != nil {
		return fmt.Errorf("write site metadata: %w", err)
	}

	// Copy default theme into new site
	copyTheme(defaultTheme, filepath.Dir(path+"/"+ThemePath))

	// Write default index page
	indexData := []byte(`Title: Home
ShowInFeed: false
PageType: home
---
`)
	err = os.WriteFile(filepath.Join(path, "index.dj"), indexData, 0664)
	if err != nil {
		panic(err)
	}

	// Write empty redirects file
	err = os.WriteFile(filepath.Join(path, RedirectsPath), []byte{}, 0664)
	if err != nil {
		panic(err)
	}

	return nil
}

func copyTheme(src fs.FS, dst string) error {
	fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		dstPath := filepath.Join(dst, path)

		if d.IsDir() {
			os.MkdirAll(dstPath, 0755)
			return nil
		}

		content, err := fs.ReadFile(src, path)
		if err != nil {
			return fmt.Errorf("read source file: %w", err)
		}

		err = os.WriteFile(dstPath, content, 0644)
		if err != nil {
			return fmt.Errorf("write dest file: %w", err)
		}

		return nil
	})

	return nil
}
