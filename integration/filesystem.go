//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
)

type file struct {
	name    string
	content string
}

func (f *file) create(root string) error {
	filePath := filepath.Join(root, f.name)
	if err := os.WriteFile(filePath, []byte(f.content), 0755); err != nil {
		return err
	}
	return nil
}

type dir struct {
	name   string
	files  []file
	subdir []dir
}

func (d *dir) create(root string) error {
	// creates itself in the fs
	dirPath := filepath.Join(root, d.name)
	if err := os.Mkdir(dirPath, 0755); err != nil {
		return err
	}

	for _, d := range d.subdir {
		if err := d.create(dirPath); err != nil {
			return err
		}
	}

	for _, f := range d.files {
		if err := f.create(dirPath); err != nil {
			return err
		}
	}

	return nil
}

type fsStructure struct {
	rootPath string
	dirs     []dir
	files    []file
}

func (f *fsStructure) setDirs(dirs []dir) *fsStructure {
	f.dirs = dirs
	return f
}

func (f *fsStructure) create() error {
	// create directories under the temporary dir
	for _, d := range f.dirs {
		if err := d.create(f.rootPath); err != nil {
			return err
		}
	}

	// create files under the temporary dir
	for _, d := range f.files {
		dirPath := filepath.Join(f.rootPath, d.name)
		if err := os.Mkdir(dirPath, 0755); err != nil {
			return err
		}
	}

	return nil
}

func newFsStructure(t *testing.T) *fsStructure {
	return &fsStructure{
		rootPath: t.TempDir(),
	}
}
