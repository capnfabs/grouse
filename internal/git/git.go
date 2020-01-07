package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/capnfabs/grouse/internal/out"
	"github.com/kballard/go-shellquote"
	au "github.com/logrusorgru/aurora"
)

type Repository struct {
	RootDir string
}

type Hash string

const NilHash = Hash("")

type ResolvedCommit struct {
	repo *Repository
	hash Hash
}

type ResolvedUserRef struct {
	Commit  ResolvedCommit
	UserRef string
}

type Worktree struct {
	Location string
}

func (r *ResolvedUserRef) String() string {
	return fmt.Sprintf("%s (%s)", au.Blue(r.UserRef), au.Yellow(r.Commit.hash[:7]))
}

func OpenRepository(repoDir string) (*Repository, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = repoDir
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	rootDir := strings.TrimSpace(stdout.String())
	return &Repository{
		RootDir: rootDir,
	}, nil
}

type cmdResult struct {
	stderr string
	stdout string
	err    error
}

func runCommand(workDir string, args ...string) cmdResult {
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
	return cmdResult{
		stderr: stderr,
		stdout: stdout,
		err:    err,
	}

}

func (r *Repository) runCommand(args ...string) cmdResult {
	return runCommand(r.RootDir, args...)
}

func (w *Worktree) runCommand(args ...string) cmdResult {
	return runCommand(w.Location, args...)
}

func GetRelativeLocation(currentDir string) (string, error) {
	cmd := runCommand(currentDir, "git", "rev-parse", "--show-prefix")
	if cmd.err != nil {
		return "", cmd.err
	}
	return cmd.stdout, nil
}

func (r *Repository) ResolveCommit(ref string) (*ResolvedUserRef, error) {
	cmd := r.runCommand("git", "rev-parse", "--verify", ref+"^{commit}")
	if cmd.err != nil {
		return nil, cmd.err
	}
	commit := ResolvedCommit{
		repo: r,
		hash: Hash(cmd.stdout),
	}
	return &ResolvedUserRef{
		commit,
		ref,
	}, nil
}

func (r *Repository) AddWorktree(dst string) (*Worktree, error) {
	cmd := r.runCommand("git", "worktree", "add", "--detach", dst)
	if cmd.err != nil {
		return nil, cmd.err
	}
	return &Worktree{
		Location: dst,
	}, nil
}

func (w *Worktree) Checkout(commit *ResolvedCommit) error {
	cmd := w.runCommand("git", "checkout", "--detach", string(commit.hash))
	return cmd.err
}

func (w *Worktree) Remove() error {
	cmd := w.runCommand("git", "worktree", "remove", w.Location)
	return cmd.err
}

var (
	ErrRepoExists = errors.New("Repo already exists")
)

func NewRepository(dst string) (*Repository, error) {
	_, err := OpenRepository(dst)
	if err == nil {
		return nil, ErrRepoExists
	}
	cmd := runCommand(dst, "git", "init")
	if cmd.err != nil {
		return nil, cmd.err
	}
	return &Repository{
		RootDir: dst,
	}, nil
}

func (r *Repository) CommitEverythingInWorktree(message string) (Hash, error) {
	// TODO: if your build produces a .gitignore file, everything that it
	// references will be excluded from the commit. It probably shouldn't be. 😅
	cmd := r.runCommand("git", "add", ".")
	if cmd.err != nil {
		return NilHash, cmd.err
	}

	cmd = r.runCommand("git", "commit", "--message", message)
	if cmd.err != nil {
		return NilHash, cmd.err
	}

	cmd = r.runCommand("git", "rev-parse", "--verify", "HEAD")
	if cmd.err != nil {
		return NilHash, cmd.err
	}
	return Hash(cmd.stdout), nil
}
