package main

import "github.com/capnfabs/hugo-diff/internal/pkg"

func main() {
	pkg.GitCommand = "difftool"
	pkg.Main()
}
