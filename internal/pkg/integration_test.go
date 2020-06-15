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
	"github.com/capnfabs/grouse/internal/out"
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
	label  string
	src    string
	ref1   string
	ref2   string
	args   string
	subdir string
}

func (tc TestCase) subdirectory(dir string) TestCase {
	tc.subdir = dir
	return tc
}

func (tc TestCase) commits(sha1, sha2 string) TestCase {
	tc.ref1 = sha1
	tc.ref2 = sha2
	return tc
}

func (tc TestCase) outfile(label string) TestCase {
	tc.label = label
	return tc
}

func tc(zipFile string) TestCase {
	label := strings.TrimSuffix(zipFile, ".zip")
	return TestCase{
		label: label,
		src:   zipFile,
		ref1:  "HEAD^",
		ref2:  "HEAD",
		args:  "",
	}
}

var TestCases []TestCase = []TestCase{
	tc("nested-submods.zip"),
	tc("nested-submod-deinit.zip"),
	tc("submod-deinit.zip"),
	tc("everything-in-subdir.zip").subdirectory("hugodir"),
	tc("unirepo-gitinfo.zip").commits("742de0a", "353bfcb").outfile("remove-submods"),
	tc("unirepo-gitinfo.zip"),
}

// findSubDir returns the path to the single directory within `dir`.
func findSubDir(t *testing.T, dir string) string {
	files, err := ioutil.ReadDir(dir)
	assert.Nil(t, err)

	filtered := []os.FileInfo{}
	names := []string{}
	for _, f := range files {
		if f.Name() != "__MACOSX" {
			filtered = append(filtered, f)
			names = append(names, f.Name())
		}
	}
	if len(filtered) != 1 {
		panic(fmt.Sprintf("Got more than 1 directory in zipfile: [%s]", strings.Join(names, ", ")))
	}
	assert.Len(t, filtered, 1)
	subdir := filtered[0]
	assert.True(t, subdir.IsDir())
	return path.Join(dir, subdir.Name())
}

func buildContext(tc *TestCase, repoDir string) cmdArgs {
	return cmdArgs{
		repoDir:      repoDir,
		diffCommand:  "diff",
		commits:      []string{tc.ref1, tc.ref2},
		diffArgs:     []string{},
		noPager:      false,
		buildArgs:    []string{},
		debug:        false,
		keepWorktree: false,
	}
}

func captureOutput(f func() error) ([]byte, []byte, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rout, wout, _ := os.Pipe()
	rerr, werr, _ := os.Pipe()
	// TODO check errors

	os.Stdout = wout
	os.Stderr = werr

	out.Reinit(false)

	outC := make(chan []byte)
	errC := make(chan []byte)

	go func() {
		data, err := ioutil.ReadAll(rout)
		if err != nil {
			panic(err)
		}
		rout.Close()
		outC <- data
	}()

	go func() {
		data, err := ioutil.ReadAll(rerr)
		if err != nil {
			panic(err)
		}
		rerr.Close()
		errC <- data
	}()

	restore := func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		out.Reinit(false)
	}

	call := func() error {
		defer restore()
		return f()
	}

	retVal := call()

	wout.Close()
	werr.Close()

	stdout := <-outC
	stderr := <-errC
	return stdout, stderr, retVal
}

var SKIPS []*regexp.Regexp = skipRegexes()

func skipRegexes() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`Total in \d+ ms`),
		// It's a log line for the current date :-/
		regexp.MustCompile(`WARN \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`),
		// Under limited (and unknown) circumstances, commit SHAs can change,
		// so ignore them in diffs.
		regexp.MustCompile(`^index [a-f0-9]{7}\.\.[a-f0-9]{7} 100644$`),
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
	errPath := path.Join(wd, "../../test-fixtures", tc.label+"-err.txt")

	inputDir := findSubDir(t, tempDir)
	if tc.subdir != "" {
		inputDir = path.Join(inputDir, tc.subdir)
	}
	fmt.Println("Test input directory is", inputDir)

	stdout, stderr, err := captureOutput(func() error {
		return runMain(git.NewGit(), buildContext(&tc, inputDir))
	})
	if err != nil {
		fmt.Println(string(stdout))
		require.Nil(t, err)
	}
	fmt.Println("Stdout is", len(stdout), "bytes")

	if out, ok := os.LookupEnv("WRITE_TEST_OUTPUT"); ok && out == "1" {
		fmt.Println("Writing stdout to", outputPath)
		fmt.Println("Writing stderr to", errPath)
		outFile, err := os.Create(outputPath)
		require.Nil(t, err)
		errFile, err := os.Create(errPath)
		require.Nil(t, err)
		outFile.Write(stdout)
		errFile.Write(stderr)
	} else {
		fmt.Println("Comparing stdout/stderr to historical values:")
		fmt.Println("- stdout:", outputPath)
		fmt.Println("- stderr:", errPath)

		checkFiltered(t, outputPath, string(stdout))
		checkFiltered(t, errPath, string(stderr))
	}
}

func checkFiltered(t *testing.T, referenceFilePath string, actualValue string) {
	referenceFile, err := os.Open(referenceFilePath)
	require.Nil(t, err)
	content, err := ioutil.ReadAll(referenceFile)
	require.Nil(t, err)
	require.Equal(t, filterLines(string(content)), filterLines(actualValue))
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
