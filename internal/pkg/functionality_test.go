package pkg

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/capnfabs/grouse/test/aferobilly"
	"github.com/spf13/afero"

	qt "github.com/frankban/quicktest"
)

/* test ideas:

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

func TestResolvesGitPathCorrectly(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		repo            string
		startingCwd     string
		expectedSubPath string
	}{
		{"tiny.zip", "", ""},
		{"tiny.zip", "src", "src"},
		{"tiny.zip", "src/content", "src/content"},
		{"tiny-norepo.zip", "", "ERROR"},
		{"tiny-norepo.zip", "src/content", "ERROR"},
	}
	for i, tc := range cases {
		c.Run(fmt.Sprintf("%d_%s", i, tc.repo), func(c *qt.C) {
			// Setup: extract temporary directory
			tempDir, err := ioutil.TempDir("", "grouse_test")
			c.Assert(err, qt.IsNil)
			wd, _ := os.Getwd()
			// This is _way_ easier to write than doing it manually within Go,
			// but it means we have to use the filesystem and it only works on
			// unix-y OSes. The "right way" to build this would be to add a
			// zip-file backend for Billy.
			cmd := exec.Command("unzip", path.Join(wd, "../../test-fixtures", tc.repo), "-d", tempDir)
			err = cmd.Run()
			c.Assert(err, qt.IsNil)
			fmt.Println("Test input directory is", path.Join(tempDir, "input"))

			startingDir := path.Join(tempDir, "input", tc.startingCwd)
			repo, relDir, err := findRepoInPath(startingDir)
			if tc.expectedSubPath == "ERROR" {
				c.Check(repo, qt.IsNil)
				c.Check(relDir, qt.Equals, "")
				c.Check(err, qt.ErrorMatches, ".*repository does not exist.*")
			} else {
				c.Check(repo, qt.Not(qt.IsNil))
				c.Check(relDir, qt.Equals, tc.expectedSubPath)
				c.Check(err, qt.IsNil)
			}
		})
	}
}
