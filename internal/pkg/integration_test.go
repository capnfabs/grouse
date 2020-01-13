package pkg

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/capnfabs/grouse/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/* test ideas:
- A bunch of end-2-end tests:
  - Normal repo
  - Really big repo with lots of files
  - A directory without a git repo
  - A repo with submodules in use in the tree, without submodules available
  - A repo that uses the git extensions to Hugo
*/

type TestCase struct {
	label string
	src   string
	ref1  string
	ref2  string
	args  string
	out   string
}

var TestCases []TestCase = []TestCase{
	TestCase{
		label: "000",
		src:   "tiny-src.zip",
		ref1:  "HEAD^",
		ref2:  "tawny-shouldered-podargus",
		args:  "",
		out:   "tiny-src-a.txt",
	},
}

// findSubDir returns the path to the single directory within `dir`.
func findSubDir(t *testing.T, dir string) string {
	files, err := ioutil.ReadDir(dir)
	assert.Nil(t, err)
	assert.Len(t, files, 1)
	subdir := files[0]
	assert.True(t, subdir.IsDir())
	return path.Join(dir, subdir.Name())
}

func buildContext(tc *TestCase, repoDir string) cmdArgs {
	return cmdArgs{
		repoDir:      repoDir,
		diffCommand:  "diff",
		commits:      []string{tc.ref1, tc.ref2},
		diffArgs:     []string{},
		buildArgs:    []string{},
		debug:        false,
		keepWorktree: false,
	}
}

func captureOutput(f func() error) ([]byte, error) {
	oldStdout := os.Stdout

	rout, wout, _ := os.Pipe()
	// TODO check error

	os.Stdout = wout

	outC := make(chan []byte)

	go func() {
		data, err := ioutil.ReadAll(rout)
		if err != nil {
			panic(err)
		}
		rout.Close()
		outC <- data
	}()

	restore := func() {
		os.Stdout = oldStdout
	}

	call := func() error {
		defer restore()
		return f()
	}

	retVal := call()

	wout.Close()

	stdout := <-outC
	return stdout, retVal
}

func runTest(t *testing.T, tc TestCase) {
	// Setup: extract temporary directory
	tempDir, err := ioutil.TempDir("", "grouse_test")
	require.Nil(t, err)
	wd, _ := os.Getwd()
	// This is _way_ easier to write than doing it manually within Go,
	// but it means it only works on unix-y OSes.
	cmd := exec.Command("unzip", path.Join(wd, "../../test-fixtures", tc.src), "-d", tempDir)
	require.Nil(t, cmd.Run())

	outputPath := path.Join(wd, "../../test-fixtures", tc.out)

	inputDir := findSubDir(t, tempDir)
	fmt.Println("Test input directory is", inputDir)

	stdout, err := captureOutput(func() error {
		return runMain(git.NewGit(), buildContext(&tc, inputDir))
	})
	require.Nil(t, err)
	fmt.Println("Stdout is", len(stdout), "bytes")

	if out, ok := os.LookupEnv("WRITE_TEST_OUTPUT"); ok && out == "1" {
		fmt.Println("Writing stdout to", outputPath)
		file, err := os.Create(outputPath)
		require.Nil(t, err)
		file.Write(stdout)
	} else {
		fmt.Println("Comparing stdout to historical value", outputPath)
		file, err := os.Open(outputPath)
		require.Nil(t, err)
		content, err := ioutil.ReadAll(file)
		require.Nil(t, err)
		require.Equal(t, content, stdout)
	}
}

func TestEnd2End(t *testing.T) {
	for _, tc := range TestCases {
		t.Run(tc.label, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}
