package main

import (
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

//go:embed theme
var defaultTheme embed.FS

func makeSite(path string, meta SiteMetadata) error {
	// Create web root dir
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("make site: %w", err)
	}

	// Create site metadata file
	metaFilePath := filepath.Join(path, SiteFileName)
	metaFile, err := os.Create(metaFilePath)
	if err != nil {
		return fmt.Errorf("create site metadata: %w", err)
	}
	defer metaFile.Close()

	metaEncoder := toml.NewEncoder(metaFile)
	err = metaEncoder.Encode(meta)
	if err != nil {
		return fmt.Errorf("write site metadata: %w", err)
	}

	// Copy default theme into new site
	copyTheme(defaultTheme, path)

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

		err = ioutil.WriteFile(dstPath, content, 0644)
		if err != nil {
			return fmt.Errorf("write dest file: %w", err)
		}

		return nil
	})

	os.Rename(filepath.Join(dst, "theme"), filepath.Join(dst, "_theme"))
	return nil
}
