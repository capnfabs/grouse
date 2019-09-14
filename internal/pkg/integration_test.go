package pkg

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/spf13/afero"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

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

func TestPreBuildSetup(t *testing.T) {
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s@%s", tc.fileTreeZip, tc.hash), func(t *testing.T) {
			c := qt.New(t)
			// Setup: extract temporary directory
			tempDir, err := ioutil.TempDir("", "hugo_diff_test")
			c.Assert(err, qt.IsNil)
			wd, _ := os.Getwd()
			cmd := exec.Command("unzip", path.Join(wd, "../../test-fixtures", tc.fileTreeZip), "-d", tempDir)
			err = cmd.Run()
			c.Assert(err, qt.IsNil)
			fmt.Println(path.Join(tempDir, "input"))

			// Run application code
			repo, err := git.PlainOpen(path.Join(tempDir, "input"))
			c.Assert(err, qt.IsNil)
			ref, err := resolveHash(repo, plumbing.NewHash(tc.hash))
			c.Assert(err, qt.IsNil)
			fmt.Println(ref, err)
			outputDir := path.Join(tempDir, "output")
			err = extractCommitToDirectory(ref, outputDir)
			c.Assert(err, qt.IsNil)

			// Check everything
			zipFile, err := zip.OpenReader(path.Join(wd, "../../test-fixtures", tc.expectedOutputZip))
			// First enumerate all files on filesystem
			fs := afero.NewBasePathFs(afero.NewOsFs(), outputDir)
			af := &afero.Afero{Fs: fs}
			paths := enumeratePaths(af, ".")
			expectedOutputPaths := []string{}
			for _, file := range zipFile.File {
				p := path.Clean(file.Name)
				if p == "output" {
					// It's the root, make it blank so that it plays nice with
					// the temp fs
					p = "."
				}
				if strings.HasPrefix(p, "output/") {
					p = strings.Replace(p, "output/", "", 1)
				}
				expectedOutputPaths = append(expectedOutputPaths, p)
			}
			c.Assert(paths, qt.ContentEquals, expectedOutputPaths)
		})
	}
}

func TestPostBuildPreDiffProcessing(t *testing.T) {

}
