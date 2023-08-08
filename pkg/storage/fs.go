package storage

import "os"

type fs struct{}

func (fs *fs) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (fs *fs) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (fs *fs) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (fs *fs) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func (fs *fs) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}
