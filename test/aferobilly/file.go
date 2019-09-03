package aferobilly

import (
	"github.com/spf13/afero"
	"gopkg.in/src-d/go-billy.v4"
)

type billyFile struct {
	afero.File
}

// Lock implements billy.File.Lock.
func (f *billyFile) Lock() error {
	return billy.ErrNotSupported
}

// Unlock implements billy.File.Unlock.
func (f *billyFile) Unlock() error {
	return billy.ErrNotSupported
}
