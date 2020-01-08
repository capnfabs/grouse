package pkg

import (
	"testing"

	"github.com/capnfabs/grouse/test/aferobilly"
	"github.com/spf13/afero"

	qt "github.com/frankban/quicktest"
)

/* test ideas:
- A bunch of end-2-end tests:
  - Normal repo
  - Really big repo with lots of files
  - A directory without a git repo
  - A repo with submodules in use in the tree, without submodules available
  - A repo that uses the git extensions to Hugo
*/

func useMemFs(fs afero.Fs) func() {
	oldFs := AppFs
	AppFs = fs
	return func() {
		AppFs = oldFs
	}
}

func TestEraseDirectoryExceptRootDotGit(t *testing.T) {
	c := qt.New(t)

	fs := afero.NewMemMapFs()
	defer useMemFs(fs)()

	fs.Mkdir("/src", 0755)
	fs.MkdirAll("/src/x/.y", 0755)
	fs.MkdirAll("/src/x/content", 0755)
	fs.MkdirAll("/src/.git/x", 0755)

	af := &afero.Afero{Fs: fs}
	af.WriteFile("/src/x/.y/foo", []byte("hello there"), 0644)
	af.WriteFile("/src/x/content/source.txt", []byte("here is some content"), 0644)
	af.WriteFile("/src/x/content/.hidden", []byte("here is a hidden file"), 0644)
	af.WriteFile("/src/.git/file1", []byte("file1"), 0644)
	af.WriteFile("/src/.git/x/file2", []byte("file2"), 0644)

	eraseDirectoryExceptRootDotGit("/src/")

	paths := aferobilly.EnumeratePaths(af, "/src")
	c.Assert(paths, qt.ContentEquals, []string{
		"/src",
		"/src/.git",
		"/src/.git/x",
		"/src/.git/file1",
		"/src/.git/x/file2",
	})
	content, _ := af.ReadFile("/src/.git/file1")
	c.Assert(content, qt.DeepEquals, []byte("file1"))

	content, _ = af.ReadFile("/src/.git/x/file2")
	c.Assert(content, qt.DeepEquals, []byte("file2"))
}
