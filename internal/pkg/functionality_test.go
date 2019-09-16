package pkg

import (
	"fmt"
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

type flags map[string]interface{}

func (f flags) GetBool(key string) (bool, error) {
	return f[key].(bool), nil
}

func (f flags) GetString(key string) (string, error) {
	return f[key].(string), nil
}

func (f flags) Args() []string {
	return f["_args"].([]string)
}

func defaultFlags() flags {
	return flags{
		"diffargs":  "--potato 'excellent'",
		"buildargs": "--carrot",
		"tool":      true,
		"_args":     []string{"b1234553", "HEAD^"},
	}
}

func TestCreateContextBaseCase(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	context, err := createContext(f)
	c.Check(err, qt.IsNil)
	c.Check(context.repoDir, qt.Not(qt.Equals), "")
	c.Check(context.diffCommand, qt.Equals, "difftool")
	c.Check(context.commits, qt.DeepEquals, []string{"b1234553", "HEAD^"})
	c.Check(context.diffArgs, qt.DeepEquals, []string{"--potato", "excellent"})
	c.Check(context.buildArgs, qt.DeepEquals, []string{"--carrot"})
}

func TestCreateContextTool(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["tool"] = false
	context, err := createContext(f)
	c.Check(err, qt.IsNil)
	c.Check(context.diffCommand, qt.Equals, "diff")
}

func TestCreateContextBadEscapedDiffArgs(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["diffargs"] = "--hello='wut" // missing single quote
	context, err := createContext(f)
	c.Check(context, qt.IsNil)
	c.Check(err, qt.ErrorMatches, `.*--diffargs:.*`)
}

func TestCreateContextBadEscapedBuildArgs(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["buildargs"] = "--hello=\"wut" // missing double quote
	context, err := createContext(f)
	c.Check(context, qt.IsNil)
	c.Check(err, qt.ErrorMatches, `.*--buildargs:.*`)
}

func TestHandlesOneCommit(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["_args"] = []string{"b1234553"}
	context, err := createContext(f)
	c.Check(err, qt.IsNil)
	c.Check(context.commits, qt.DeepEquals, []string{"b1234553", "HEAD"})
}

func TestHandlesWrongNumberOfCommits(t *testing.T) {
	c := qt.New(t)
	for _, testcase := range [][]string{[]string{}, []string{"b1234553", "HEAD^^", "HEAD"}} {
		subtest := fmt.Sprintf("With%dArgs", len(testcase))
		c.Run(subtest, func(c *qt.C) {
			f := defaultFlags()
			f["_args"] = testcase
			context, err := createContext(f)
			c.Check(context, qt.IsNil)
			c.Check(err, qt.ErrorMatches, `Requires one or two git references.*got \d`)
		})
	}
}
