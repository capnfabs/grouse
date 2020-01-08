package git

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/capnfabs/grouse/internal/out"
	"github.com/cf-guardian/guardian/kernel/fileutils"
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

func (r *ResolvedCommit) String() string {
	return string(r.hash[:7])
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

func findSubmoduleGitRepos(rootDir string) {
	files, err := ioutil.ReadDir(rootDir)
	if err != nil {
		panic(err)
	}
	// Look for a config file
	for _, file := range files {
		if file.Name() == "config" && (file.Mode()&os.ModeType) == 0 {
			// Oh! We have one!
			p := path.Join(rootDir, "config")
			out.Debugln("Unsetting core.worktree from file", p)
			cmd := runCommand("/", "git", "config", "--file", p, "--unset", "core.worktree")
			if cmd.err == nil {
				// Ok it worked, no need to look in other directories
				out.Debugln("Succeeded!")
				return
			} else {
				out.Debugln("Failed, maybe not a git config file :-/")
				break
			}
		}
	}
	// Apparently not a git repo, go a level deeper
	for _, file := range files {
		if file.IsDir() {
			findSubmoduleGitRepos(path.Join(rootDir, file.Name()))
		}
	}
}

func (r *Repository) AddWorktree(dst string) (*Worktree, error) {
	cmd := r.runCommand("git", "worktree", "add", "--no-checkout", "--detach", dst)
	if cmd.err != nil {
		return nil, cmd.err
	}

	wt := Worktree{
		Location: dst,
	}

	// OPTIMISATION

	// Submodule wizardry!

	// First: get the modules path for the base repo.
	// Note that this is potentially extremely edge-casey -- old versions of git
	// store their modules in-directory. This effectively assumes a _newer_
	// version of git.
	cmd = r.runCommand("git", "rev-parse", "--git-path", "modules")
	if cmd.err != nil {
		panic(cmd.err)
	}

	rootModules := cmd.stdout
	// Canonicalise it.
	if !path.IsAbs(rootModules) {
		rootModules = path.Join(r.RootDir, rootModules)
	}

	// Now get the modules path for the worktree.
	cmd = wt.runCommand("git", "rev-parse", "--git-path", "modules")
	if cmd.err != nil {
		panic(cmd.err)
	}
	wtModules := cmd.stdout

	// NOTE: I never observed relative paths with worktrees but I'm still
	// worried about it.
	if !path.IsAbs(wtModules) {
		wtModules = path.Join(wt.Location, wtModules)
	}

	out.Debugln("Hack-copying modules from", rootModules, "to", wtModules)

	// I _think_ the way to do this is:
	// 1. Find submodules that are init'd (`git submodule status` isn't prefixed with a '-')
	// 2. those will be automatically init'd in new worktree, but the links won't be in place (don't know how to detect this?)
	// 3. for each of those submodules, copy the object storage across, and modify the `config` file
	err1 := fileutils.New().Copy(wtModules, rootModules)
	if err1 != nil {
		return nil, err1
	}

	// Manually edit the config files to erase the worktree in them
	findSubmoduleGitRepos(wtModules)

	return &wt, nil
}

func (w *Worktree) Checkout(commit *ResolvedCommit) error {
	cmd := w.runCommand("git", "checkout", "--detach", string(commit.hash))
	if cmd.err != nil {
		return cmd.err
	}

	// Remove stuff that wasn't explicitly checked in.
	// -x is ??
	// -ff is to also remove nested git repos (submodules).
	cmd = w.runCommand("git", "clean", "-ffxd")
	if cmd.err != nil {
		return cmd.err
	}

	// Checkout submodules
	cmd = w.runCommand("git", "submodule", "update")
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

	cmd = r.runCommand("git", "commit", "--message", message, "--quiet", "--allow-empty")
	if cmd.err != nil {
		return NilHash, cmd.err
	}

	cmd = r.runCommand("git", "rev-parse", "--verify", "HEAD")
	if cmd.err != nil {
		return NilHash, cmd.err
	}
	return Hash(cmd.stdout), nil
}
