package checkout

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/capnfabs/grouse/test/aferobilly"
	qt "github.com/frankban/quicktest"
	"github.com/spf13/afero"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

func TestExtractFilesAtCommitToDir(t *testing.T) {
	return
	c := qt.New(t)
	fs := afero.NewMemMapFs()

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

	hash, err := resolveHash(repo, sha)
	if err != nil {
		panic(err)
	}

	af.Mkdir("/dst", 0755)

	extractCommitToDirectoryWithContext(&extractCommitContext{
		fs: fs,
	}, hash, "/dst")

	paths := aferobilly.EnumeratePaths(af, "/dst")
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

var cases = []struct {
	fileTreeZip       string
	hash              string
	expectedOutputZip string
}{
	// Features of this repo:
	// - No submodules
	// - has uncommitted changes (`config.toml`)
	// - has files in the working tree that are gitignored (`GITIGNORED`)
	{"nomodules-src.zip", "b789d11b2eaa2e3e4c1f942b2580492274fd32a4", "nomodules-b789d11.zip"},
	{"nomodules-src.zip", "cb28ff96c995f6e0378347139f1188a7bf77964a", "nomodules-cb28ff9.zip"},
	// Features of this repo:
	// - Has submodules
	// - One submodule currently in use is in the working tree
	// - One submodule no longer in use is in the history, but not in the working tree.
	// This first commit has all relevant submodules still in the working tree
	{"themechange-src.zip", "1da5eec49d7fa3529b553055d86fe801714846f1", "themechange-1da5eec4.zip"},
	// This second commit uses a theme which is _no longer_ in the working tree.
	{"themechange-src.zip", "4367feb0439721ec67cf4175e59454326643d951", "themechange-4367feb0.zip"},
}

func loadAnankeTheme(t *testing.T) *git.Repository {
	c := qt.New(t)
	tempDir, err := ioutil.TempDir("", "theme-ananke")
	c.Assert(err, qt.IsNil)
	wd, _ := os.Getwd()
	cmd := exec.Command("unzip", path.Join(wd, "../../test-fixtures", "gohugo-theme-ananke.zip"), "-d", tempDir)
	err = cmd.Run()
	c.Assert(err, qt.IsNil)
	themeLoc := path.Join(tempDir, "gohugo-theme-ananke")
	fmt.Println("Theme location:", themeLoc)
	repo, err := git.PlainOpen(themeLoc)
	c.Assert(err, qt.IsNil)
	return repo
}

func makeTestContext(t *testing.T) *extractCommitContext {
	// TODO: it's slow running this on every test.
	repo := loadAnankeTheme(t)
	return &extractCommitContext{
		fs: afero.NewMemMapFs(),
		submoduleCloner: func(_ storage.Storer, _ billy.Filesystem, o *git.CloneOptions) (*git.Repository, error) {
			return repo, nil
		},
	}
}

func TestWithZipFiles(t *testing.T) {
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s@%s", tc.fileTreeZip, tc.hash), func(t *testing.T) {
			c := qt.New(t)
			// Setup: extract temporary directory
			tempDir, err := ioutil.TempDir("", "grouse_test")
			c.Assert(err, qt.IsNil)
			wd, _ := os.Getwd()
			// This is _way_ easier to write than doing it manually within Go,
			// but it means we have to use the filesystem and it only works on
			// unix-y OSes. The "right way" to build this would be to add a
			// zip-file backend for Billy.
			cmd := exec.Command("unzip", path.Join(wd, "../../test-fixtures", tc.fileTreeZip), "-d", tempDir)
			err = cmd.Run()
			c.Assert(err, qt.IsNil)
			fmt.Println("Test input directory is", path.Join(tempDir, "input"))
			context := makeTestContext(t)

			// Run application code
			repo, err := git.PlainOpen(path.Join(tempDir, "input"))
			c.Assert(err, qt.IsNil)
			ref, err := ResolveHash(repo, plumbing.NewHash(tc.hash))
			c.Assert(err, qt.IsNil)
			outputDir := "/output/"
			err = extractCommitToDirectoryWithContext(context, ref, outputDir)
			c.Assert(err, qt.IsNil)

			// Check everything
			zipFile, err := zip.OpenReader(path.Join(wd, "../../test-fixtures", tc.expectedOutputZip))
			// First enumerate all files on filesystem
			af := &afero.Afero{Fs: context.fs}
			paths := aferobilly.EnumeratePaths(af, "/output/")

			// These output paths are equal because the zip file unzips to an "output" directory.
			// Which is gross, but oh well.
			expectedOutputPaths := []string{}
			for _, file := range zipFile.File {
				p := path.Join("/", file.Name)
				expectedOutputPaths = append(expectedOutputPaths, p)
			}
			c.Assert(paths, qt.ContentEquals, expectedOutputPaths)
		})
	}
}

func TestCommitDereferencing(t *testing.T) {
	c := qt.New(t)
	var cases = []struct {
		userRef string
		hash    string
		label   string
	}{
		{"master", "d41f106361f57e4fe349241ebb8794ae9c382222", "master (d41f106)"},
		{"HEAD", "f463b3e998a55fb3a64e78412a3e02fec6db00a0", "HEAD (f463b3e)"},
		{"HEAD^", "d41f106361f57e4fe349241ebb8794ae9c382222", "HEAD^ (d41f106)"},
		{"tawny-shouldered-podargus", "f463b3e998a55fb3a64e78412a3e02fec6db00a0", "tawny-shouldered-podargus (f463b3e)"},
		{"nope", "", ""},
		{"HEAD^11", "", ""},
	}

	// Setup: extract temporary directory
	tempDir, err := ioutil.TempDir("", "grouse_test")
	c.Assert(err, qt.IsNil)
	wd, _ := os.Getwd()
	// This is _way_ easier to write than doing it manually within Go,
	// but it means we have to use the filesystem and it only works on
	// unix-y OSes. The "right way" to build this would be to add a
	// zip-file backend for Billy.
	cmd := exec.Command("unzip", path.Join(wd, "../../test-fixtures", "tiny.zip"), "-d", tempDir)
	err = cmd.Run()
	c.Assert(err, qt.IsNil)
	fmt.Println("Test input directory is", path.Join(tempDir, "input"))

	for _, tc := range cases {
		c.Run(tc.userRef, func(c *qt.C) {
			// Run application code
			repo, err := git.PlainOpen(path.Join(tempDir, "input"))
			c.Assert(err, qt.IsNil)
			ref, err := ResolveUserRef(repo, tc.userRef)
			if tc.hash != "" {
				c.Assert(err, qt.IsNil)
				c.Check(ref.Hash().String(), qt.Equals, tc.hash)
				c.Check(ref.Repo(), qt.Equals, repo)
				c.Check(fmt.Sprintf("%v", ref), qt.Equals, tc.label)
			} else {
				c.Check(ref, qt.IsNil)
				// Error messages here come from somewhere deep within go-git,
				// so we shouldn't test them.
				c.Check(err, qt.Not(qt.IsNil))
			}
		})
	}
}
