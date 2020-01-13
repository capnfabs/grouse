package main

import (
	"io/ioutil"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestFlagVisibilityInHelpText(t *testing.T) {
	c := qt.New(t)

	rootCmd.SetArgs([]string{"--help"})

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	main()

	w.Close()
	out, _ := ioutil.ReadAll(r)
	os.Stdout = origStdout

	help := string(out)

	c.Check(help, qt.Contains, "--diffargs")
	c.Check(help, qt.Contains, "--debug")
	c.Check(help, qt.Contains, "--tool")
	c.Check(help, qt.Contains, "-t")
	c.Check(help, qt.Not(qt.Contains), "keep-worktree")
}
