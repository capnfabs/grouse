package pkg

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
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
}

var TestCases []TestCase = []TestCase{
	TestCase{
		label: "tiny-simple-branch-diff",
		src:   "tiny.zip",
		ref1:  "HEAD^",
		ref2:  "tawny-shouldered-podargus",
		args:  "",
	},
	TestCase{
		label: "nomodules-two-commit-diff",
		src:   "nomodules.zip",
		ref1:  "b789d11b2eaa2e3e4c1f942b2580492274fd32a4",
		ref2:  "10730e4c7f320144af7055b37daecd240b4b0b72",
		args:  "",
	},
	TestCase{
		label: "nomodules-empty-diff",
		src:   "nomodules.zip",
		ref1:  "10730e4c7f320144af7055b37daecd240b4b0b72",
		ref2:  "HEAD",
		args:  "",
	},
	TestCase{
		// Change theme from 'ananke' to 'piercer'
		label: "themechange-change-theme",
		src:   "themechange.zip",
		ref1:  "4367feb0439721ec67cf4175e59454326643d951",
		ref2:  "3035eafa7b793b66a76b783846b67b92e0565f56",
		args:  "",
	},
	TestCase{
		// Delete 'ananke' submodule; should be a noop in terms of diff
		// but involves automatically cloning a submodule so it's probably slow.
		label: "themechange-old-theme-missing-from-tree",
		src:   "themechange.zip",
		ref1:  "3035eafa7b793b66a76b783846b67b92e0565f56",
		ref2:  "1da5eec49d7fa3529b553055d86fe801714846f1",
		args:  "",
	},
	TestCase{
		label: "nested-submodules-everything-present",
		src:   "nested-submodules-everything-present.zip",
		ref1:  "HEAD^",
		ref2:  "HEAD",
		args:  "",
	},
	TestCase{
		// This simulates the case where some nested submodules are missing
		// from the tree.
		label: "nested-submodules-missing-submodules",
		src:   "nested-submodules-missing-submodules.zip",
		ref1:  "HEAD^",
		ref2:  "HEAD",
		args:  "",
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
		gitArgs:      []string{},
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

var SKIPS []*regexp.Regexp = skipRegexes()

func skipRegexes() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`Total in \d+ ms`),
		// It's a log line for the current date :-/
		regexp.MustCompile(`WARN \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`),
		// These two are only issues in the nested-submodules tests. Maybe
		// we should modify those tests?
		// Under limited (and unknown) circumstances, commit SHAs can change,
		// so ignore them in diffs.
		regexp.MustCompile(`^index [a-f0-9]{7}\.\.[a-f0-9]{7} 100644$`),
		// This has a hash in it too :-/
		regexp.MustCompile(`https://example\.com/css/site\.min`),
	}
}

func shouldSkipLine(line string) bool {
	for _, re := range SKIPS {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func filterLines(content string) string {
	filtered := []string{}
	for _, line := range strings.Split(content, "\n") {
		// This is internal timing info, skip it.
		// Ideally, we'd skip all hugo build output? It's not part of the API.
		if !shouldSkipLine(line) {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
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

	outputPath := path.Join(wd, "../../test-fixtures", tc.label+"-out.txt")

	inputDir := findSubDir(t, tempDir)
	fmt.Println("Test input directory is", inputDir)

	stdout, err := captureOutput(func() error {
		return runMain(git.NewGit(), buildContext(&tc, inputDir))
	})
	if err != nil {
		fmt.Println(string(stdout))
		require.Nil(t, err)
	}
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
		require.Equal(t, filterLines(string(content)), filterLines(string(stdout)))
	}
}

func TestEnd2End(t *testing.T) {
	labels := make(map[string]struct{})

	for _, tc := range TestCases {
		if _, ok := labels[tc.label]; ok {
			require.FailNow(t, "Multiple testcases with same label, ensure all testcases have unique names.")
		} else {
			labels[tc.label] = struct{}{}
		}
	}

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	for _, tc := range TestCases {
		t.Run(tc.label, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}
