package exec

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/capnfabs/grouse/internal/out"
	"github.com/kballard/go-shellquote"
)

type CmdResult struct {
	StdErr string
	StdOut string
	Err    error
}

type Executor func(workDir string, args ...string) CmdResult
type CommandRunner func(cmd *Cmd) error

type Cmd struct {
	*exec.Cmd
}

// Probably a better way to do this, but it works for now!
func (c *Cmd) Run(DONT_CALL_THIS string) {
	// This doesn't support test injection, so do this other thing instead.
	panic("Don't call this; use Run(cmd) instead")
}

func Command(name string, arg ...string) *Cmd {
	return &Cmd{exec.Command(name, arg...)}
}

var Run CommandRunner = func(cmd *Cmd) error {
	return cmd.Cmd.Run()
}

var Exec Executor = func(workDir string, args ...string) CmdResult {
	out.Debugln("Running Command: ", shellquote.Join(args...))
	cmd := exec.Command(args[0], args[1:]...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	cmd.Stdout = &stdoutBuf
	cmd.Dir = workDir
	err := cmd.Run()
	stderr := strings.TrimSpace(stderrBuf.String())
	stdout := strings.TrimSpace(stdoutBuf.String())
	out.Debugln("StdErr: ", stderr)
	out.Debugln("StdOut: ", stdout)
	return CmdResult{
		StdErr: stderr,
		StdOut: stdout,
		Err:    err,
	}
}

type ExitError = exec.ExitError
