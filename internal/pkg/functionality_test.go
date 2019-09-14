package pkg

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/capnfabs/grouse/test/aferobilly"
	"github.com/spf13/afero"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"

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

func enumeratePaths(af *afero.Afero, root string) []string {
	paths := []string{}
	err := af.Walk(root, func(path string, info os.FileInfo, err error) error {
		paths = append(paths, path)
		return err
	})
	if err != nil {
		panic(err)
	}
	return paths
}

func TestEraseDirectoryExceptRootDotGit(t *testing.T) {
	c := qt.New(t)
	fs := afero.NewMemMapFs()
	AppFs = fs
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

	paths := enumeratePaths(af, "/src")
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

func TestExtractFilesAtCommitToDir(t *testing.T) {
	c := qt.New(t)
	fs := afero.NewMemMapFs()
	AppFs = fs
	fs.Mkdir("/src", 0755)
	fs.MkdirAll("/src/x/.y", 0755)
	fs.MkdirAll("/src/x/content", 0755)

	af := &afero.Afero{Fs: fs}
	af.WriteFile("/src/x/.y/foo", []byte("hello there"), 0644)
	af.WriteFile("/src/x/content/source.txt", []byte("here is some content"), 0644)
	af.WriteFile("/src/x/content/.hidden", []byte("here is a hidden file"), 0644)

	storage := memory.NewStorage()

	repoRoot, err := aferobilly.NewBillyAeroFs(fs).Chroot("/src")
	files, err := repoRoot.ReadDir("/")
	if err != nil {
		panic(err)
	}
	fmt.Println("lol")
	for _, file := range files {
		fmt.Println(file.Name())
	}
	repo, err := git.Init(storage, repoRoot)
	if err != nil {
		panic(err)
	}
	wt, _ := repo.Worktree()
	_, err = wt.Add(".")
	if err != nil {
		panic(err)
	}
	sha, _ := wt.Commit("Hello here's a commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Some Person",
			Email: "some-person@example.net",
			When:  time.Now(),
		},
	})

	commit, err := repo.CommitObject(sha)
	if err != nil {
		panic(err)
	}

	af.Mkdir("/dst", 0755)
	extractFilesAtCommitToDir(wt, repo, commit, "/dst")

	paths := enumeratePaths(af, "/dst")
	c.Assert(paths, qt.ContentEquals, []string{
		"/dst",
		"/dst/x",
		"/dst/x/.y",
		"/dst/x/.y/foo",
		"/dst/x/content",
		"/dst/x/content/source.txt",
		"/dst/x/content/.hidden",
	})
	content, _ := af.ReadFile("/dst/x/content/source.txt")
	c.Assert(content, qt.DeepEquals, []byte("here is some content"))

	content, _ = af.ReadFile("/dst/x/.y/foo")
	c.Assert(content, qt.DeepEquals, []byte("hello there"))
}
