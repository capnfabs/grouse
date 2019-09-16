package pkg

import (
	"testing"

	"github.com/capnfabs/grouse/test/aferobilly"
	"github.com/spf13/afero"

	qt "github.com/frankban/quicktest"
)

/* test ideas:

Test command line parsing.
Later - test passthrough arguments as well.

Test that it works with 1 SHA and 2 SHAs

Test that the commits it runs the diff for are from the right repo

Add a couple of test repos and ensure that they work end-to-end

Test that supplying bad commit refs gives the right errors.
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
