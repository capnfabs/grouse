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
