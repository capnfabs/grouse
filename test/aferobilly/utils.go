package aferobilly

import (
	"os"
	"path"

	"github.com/spf13/afero"
)

func EnumeratePaths(af *afero.Afero, root string) []string {
	paths := []string{}
	err := af.Walk(root, func(p string, info os.FileInfo, err error) error {
		p = path.Clean(p)
		paths = append(paths, p)
		return err
	})
	if err != nil {
		panic(err)
	}
	return paths
}
