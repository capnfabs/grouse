package aferobilly

import (
	"os"

	"github.com/spf13/afero"
)

type CloseTrackingFs struct {
	openFiles map[string]struct{}
	afero.Fs
}

func NewCloseTrackingFs(fs afero.Fs) *CloseTrackingFs {
	return &CloseTrackingFs{
		map[string]struct{}{},
		fs,
	}
}

func (fs *CloseTrackingFs) OpenFiles() []string {
	keys := []string{}
	for k := range fs.openFiles {
		keys = append(keys, k)
	}
	return keys
}

type CloseCheckingFile struct {
	parent *CloseTrackingFs
	afero.File
}

func (c *CloseCheckingFile) Close() error {
	delete(c.parent.openFiles, c.Name())
	return c.File.Close()
}

func (fs *CloseTrackingFs) wrap(file afero.File, err error) (afero.File, error) {
	if err != nil {
		return nil, err
	}
	fs.openFiles[file.Name()] = struct{}{}
	return &CloseCheckingFile{
		fs,
		file,
	}, nil
}

func (fs *CloseTrackingFs) Create(name string) (afero.File, error) {
	return fs.wrap(fs.Fs.Create(name))
}

func (fs *CloseTrackingFs) Open(name string) (afero.File, error) {
	return fs.wrap(fs.Fs.Open(name))
}

func (fs *CloseTrackingFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	return fs.wrap(fs.Fs.OpenFile(name, flag, perm))
}
