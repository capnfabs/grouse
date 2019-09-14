package aferobilly

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/helper/chroot"
)

type billyAeroFs struct {
	afero.Fs
	afro afero.Afero
}

func NewBillyAeroFs(fs afero.Fs) billy.Filesystem {
	afro := afero.Afero{fs}
	return &billyAeroFs{
		Fs:   fs,
		afro: afro,
	}
}

// Capabilities implements billy.Filesystem.Capabilities.
func (b *billyAeroFs) Capabilities() billy.Capability {
	return billy.WriteCapability | billy.ReadCapability | billy.ReadAndWriteCapability | billy.SeekCapability | billy.TruncateCapability
}

// Chroot implements billy.Chroot.Chroot.
func (b *billyAeroFs) Chroot(path string) (billy.Filesystem, error) {
	return chroot.New(b, path), nil
}

// Create implements billy.Basic.Create.
func (b *billyAeroFs) Create(filename string) (billy.File, error) {
	f, err := b.Fs.Create(filename)
	return &billyFile{
		f,
	}, err
}

// Join implements billy.Basic.Join.
func (b *billyAeroFs) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// Open implements billy.Basic.Open.
func (b *billyAeroFs) Open(filename string) (billy.File, error) {
	f, err := b.Fs.Open(filename)
	return &billyFile{
		f,
	}, err
}

// OpenFile implements billy.Basic.OpenFile.
func (b *billyAeroFs) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	f, err := b.Fs.OpenFile(filename, flag, perm)
	return &billyFile{
		f,
	}, err
}

// Root implements billy.Chroot.Root.
func (b *billyAeroFs) Root() string {
	return "/"
}

// TempFile implements billy.TempFile.TempFile.
func (b *billyAeroFs) TempFile(dir, prefix string) (billy.File, error) {
	return nil, billy.ErrNotSupported
}

func (b *billyAeroFs) Lstat(filename string) (os.FileInfo, error) {
	return b.Fs.Stat(filename)
}

func (b *billyAeroFs) MkdirAll(path string, perm os.FileMode) error {
	return b.afro.MkdirAll(path, perm)
}

func (b *billyAeroFs) ReadDir(path string) ([]os.FileInfo, error) {
	return b.afro.ReadDir(path)
}

func (b *billyAeroFs) Readlink(link string) (string, error) {
	fmt.Println("readlink")
	return "", billy.ErrNotSupported
}

func (b *billyAeroFs) Symlink(target, link string) error {
	fmt.Println("symlink")
	return billy.ErrNotSupported
}
