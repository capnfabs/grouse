package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/capnfabs/grouse/internal/out"
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

func GetRelativeLocation(currentDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-prefix")
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = currentDir
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	rootDir := strings.TrimSpace(stdout.String())
	return rootDir, nil
}

func (r *Repository) ResolveCommit(ref string) (*ResolvedUserRef, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", ref+"^{commit}")
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = r.RootDir
	err := cmd.Run()
	out.Debugln("Err", stderr.String())
	out.Debugln("Out", stdout.String())
	if err != nil {
		return nil, err
	}
	commit := ResolvedCommit{
		repo: r,
		hash: Hash(strings.TrimSpace(stdout.String())),
	}
	return &ResolvedUserRef{
		commit,
		ref,
	}, nil
}

func (r *Repository) AddWorktree(dst string) (*Worktree, error) {
	cmd := exec.Command("git", "worktree", "add", "--detach", dst)
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = r.RootDir
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return &Worktree{
		Location: dst,
	}, nil
}

func (w *Worktree) Checkout(commit *ResolvedCommit) error {
	cmd := exec.Command("git", "checkout", "--detach", string(commit.hash))
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = w.Location
	return cmd.Run()
}

func (w *Worktree) Remove() error {
	cmd := exec.Command("git", "worktree", "remove", w.Location)
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = w.Location
	return cmd.Run()
}

var (
	ErrRepoExists = errors.New("Repo already exists")
)

func NewRepository(dst string) (*Repository, error) {
	_, err := OpenRepository(dst)
	if err == nil {
		return nil, ErrRepoExists
	}
	cmd := exec.Command("git", "init")
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = dst
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	return &Repository{
		RootDir: dst,
	}, nil
}

func (r *Repository) CommitEverythingInWorktree(message string) (Hash, error) {
	// TODO: if your build produces a .gitignore file, everything that it
	// references will be excluded from the commit. It probably shouldn't be. 😅
	cmd := exec.Command("git", "add", ".")
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = r.RootDir
	err := cmd.Run()
	if err != nil {
		return NilHash, err
	}

	cmd = exec.Command("git", "commit", "--message", message)
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = r.RootDir
	err = cmd.Run()
	if err != nil {
		return NilHash, err
	}

	cmd = exec.Command("git", "rev-parse", "--verify", "HEAD")
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Dir = r.RootDir
	err = cmd.Run()
	if err != nil {
		return NilHash, err
	}
	return Hash(strings.TrimSpace(stdout.String())), nil
}
