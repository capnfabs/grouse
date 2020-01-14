package git

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/capnfabs/grouse/internal/exec"
	"github.com/capnfabs/grouse/internal/out"
	"github.com/cf-guardian/guardian/kernel/fileutils"
	au "github.com/logrusorgru/aurora"
)

type Git interface {
	NewRepository(dst string) (Repository, error)
	OpenRepository(repoDir string) (Repository, error)
	GetRelativeLocation(currentDir string) (string, error)
}

type gitVersion struct {
	major int
	minor int
	patch int
}

func (v *gitVersion) isNewerThanOrEqualTo(major, minor, patch int) bool {
	if v.major > major {
		return true
	}
	if v.major == major {
		if v.minor > minor {
			return true
		}
		if v.minor == minor {
			if v.patch >= patch {
				return true
			}
		}
	}
	return false
}

var noVersion = gitVersion{0, 0, 0}

func parseVersionString(vs string) gitVersion {
	parts := strings.Split(vs, ".")
	if len(parts) != 3 {
		return noVersion
	}
	major, errA := strconv.Atoi(parts[0])
	minor, errB := strconv.Atoi(parts[1])
	patch, errC := strconv.Atoi(parts[2])
	if errA != nil || errB != nil || errC != nil {
		return noVersion
	}
	return gitVersion{major, minor, patch}
}

func NewGit() Git {
	cmd := exec.Exec("git", "version")
	submatches := regexp.MustCompile(`^git version (\d+\.\d+\.\d+)`).FindStringSubmatch(cmd.StdOut)
	var version gitVersion = noVersion
	if submatches != nil {
		version = parseVersionString(submatches[1])
	}
	return git{
		version: version,
	}
}

// Repository represents a git repository, somewhere on disk.
type Repository interface {
	RootDir() string
	ResolveCommit(ref string) (ResolvedUserRef, error)
	AddWorktree(dst string) (Worktree, error)
	CommitEverythingInWorktree(message string) (Hash, error)
	ClearSourceControlledFilesFromWorktree() error
}

type repository struct {
	rootDir string
	git_    *git
}

func (r *repository) RootDir() string {
	return r.rootDir
}

// Hash is a git SHA-1, base16 encoded
type Hash string

// NilHash is the zero value, i.e. the absence of a Hash.
const NilHash = Hash("")

// ResolvedCommit represents a commit which definitely exists within its
// associated repository.
type ResolvedCommit interface {
	Repo() Repository
	Hash() Hash
}

func (r *resolvedCommit) Repo() Repository {
	return r.repo
}
func (r *resolvedCommit) Hash() Hash {
	return r.hash
}

type resolvedCommit struct {
	repo *repository
	hash Hash
}

func (r *resolvedCommit) String() string {
	return string(r.hash[:7])
}

// ResolvedUserRef represents a user-provided commit-ish, which has then been
// resolved to a commit in the git repo. This is useful info together for
// referring back to input that the user has supplied; notably, the String()
// method returns a representation of the user's input and the SHA together.
type ResolvedUserRef interface {
	Commit() ResolvedCommit
	UserRef() string
}

type resolvedUserRef struct {
	commit  resolvedCommit
	userRef string
}

func (r *resolvedUserRef) Commit() ResolvedCommit {
	return &r.commit
}
func (r *resolvedUserRef) UserRef() string {
	return r.userRef
}

// Worktree represents a git worktree - it may or may not be the primary
// worktree for the repo or a secondary worktree.
type Worktree interface {
	Location() string
	Remove() error
	Checkout(commit ResolvedCommit) error
}

type worktree struct {
	location string
	git_     *git
}

func (w *worktree) Location() string {
	return w.location
}

func (r *resolvedUserRef) String() string {
	return fmt.Sprintf("%s (%s)", au.Blue(r.userRef), au.Yellow(r.commit.hash[:7]))
}

// OpenRepository opens an existing git repository in repoDir. If repoDir is a
// subdirectory in the repository, OpenRepository walks up the file tree to find
// the git repo.
func (g git) OpenRepository(repoDir string) (Repository, error) {
	cmd := exec.Exec(repoDir, "git", "rev-parse", "--show-toplevel")
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	rootDir := cmd.StdOut
	return &repository{
		rootDir: rootDir,
	}, nil
}

func (r *repository) runCommand(args ...string) exec.CmdResult {
	return exec.Exec(r.rootDir, args...)
}

func (w *worktree) runCommand(args ...string) exec.CmdResult {
	return exec.Exec(w.location, args...)
}

// GetRelativeLocation gets the relative path from the root of a git repo to
// currentDir. e.g. if there's a git repo in ~/hello, and currentDir is
// ~/hello/potato/tomato, returns "potato/tomato".
func (g git) GetRelativeLocation(currentDir string) (string, error) {
	cmd := exec.Exec(currentDir, "git", "rev-parse", "--show-prefix")
	if cmd.Err != nil {
		return "", cmd.Err
	}
	return cmd.StdOut, nil
}

func (r *repository) ResolveCommit(ref string) (ResolvedUserRef, error) {
	cmd := r.runCommand("git", "rev-parse", "--verify", ref+"^{commit}")
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	commit := resolvedCommit{
		repo: r,
		hash: Hash(cmd.StdOut),
	}
	return &resolvedUserRef{
		commit,
		ref,
	}, nil
}

func isFile(file os.FileInfo) bool {
	return file.Mode()&os.ModeType == 0
}

func (w *worktree) getModulesPath() string {
	cmd := w.runCommand("git", "rev-parse", "--git-path", "modules")
	if cmd.Err != nil {
		// This should always be available
		panic(cmd.Err)
	}
	wtModules := cmd.StdOut
	// NOTE: I never observed relative paths with worktrees but I'm still
	// worried about it; canonicalize as required.
	if !path.IsAbs(wtModules) {
		wtModules = path.Join(w.location, wtModules)
	}
	return wtModules
}

// prepSubmodulesForWorktree is Dark Magic that bootstraps git submodules for a
// secondary worktree so that you don't need to have internet to run
// `submodule update` within the worktree.
// HOW DOES IT WORK?
// - Copy-paste the `.git/modules` directory from the base repository to the
//   worktree (usually .git/worktrees/[name]/modules)
// - For each module, edit the `config` file to remove the `worktree` section
//	 (i.e.) disassociate the module from its original worktree.
// - Then, `git submodule update` from within the worktree _just works_ and uses
//   the existing context.
func prepSubmodulesForWorktree(baseRepo *repository, newWorktree *worktree) error {
	// First: get the modules path for the base repo.
	// Note that this is potentially edge-casey -- old versions of git store
	// their modules in the worktree for the module. This effectively assumes a
	// _newer_ version of git.
	cmd := baseRepo.runCommand("git", "rev-parse", "--git-path", "modules")
	if cmd.Err != nil {
		panic(cmd.Err)
	}

	rootModules := cmd.StdOut

	// Canonicalize it!
	if !path.IsAbs(rootModules) {
		rootModules = path.Join(baseRepo.rootDir, rootModules)
	}

	// Test that there _are_ submodules before trying anything tricky
	_, err := os.Lstat(rootModules)
	if err != nil && os.IsNotExist(err) {
		// No submodules, just chill.
		return nil
	} else if err != nil {
		// I can't imagine what error this could be...
		return err
	}

	// Now get the modules path for the worktree.
	wtModules := newWorktree.getModulesPath()

	out.Debugln("Hack-copying modules from", rootModules, "to", wtModules)

	// Not a regular error type; so use a different variable name
	// TODO: maybe don't import some random library just for this function?
	err1 := fileutils.New().Copy(wtModules, rootModules)
	if err1 != nil {
		return err1
	}

	return nil
}

func (r *repository) AddWorktree(dst string) (Worktree, error) {
	var args []string
	if r.git_.version.isNewerThanOrEqualTo(2, 9, 0) {
		args = []string{"git", "worktree", "add", "--no-checkout", "--detach", dst}
	} else {
		// --no-checkout not supported, doesn't matter much because we just
		// used it as a performance optimisation. Skip it.
		args = []string{"git", "worktree", "add", "--detach", dst}
	}
	cmd := r.runCommand(args...)
	if cmd.Err != nil {
		return nil, cmd.Err
	}

	wt := &worktree{
		location: dst,
		git_:     r.git_,
	}

	if err := prepSubmodulesForWorktree(r, wt); err != nil {
		return nil, err
	}
	return wt, nil
}

func (w *worktree) Checkout(commit ResolvedCommit) error {
	cmd := w.runCommand("git", "checkout", "--detach", string(commit.Hash()))
	if cmd.Err != nil {
		return cmd.Err
	}

	// Remove stuff that wasn't explicitly checked in.
	// -x is ??
	// -ff is to also remove nested git repos (submodules).
	cmd = w.runCommand("git", "clean", "-ffxd")
	if cmd.Err != nil {
		return cmd.Err
	}

	// Checkout submodules
	cmd = w.runCommand("git", "submodule", "update", "--recursive")
	return cmd.Err
}

func (w *worktree) Remove() error {
	if w.git_.version.isNewerThanOrEqualTo(2, 17, 0) {
		cmd := w.runCommand("git", "worktree", "remove", "--force", w.location)
		return cmd.Err
	} else {
		// TODO: manual deletion as per https://git-scm.com/docs/git-worktree/2.17.0#_description
		return nil
	}
}

var (
	// ErrRepoExists means that the given directory already has a git repository
	// in it, and a new repository can't be created there.
	ErrRepoExists = errors.New("Repo already exists")
)

type git struct {
	version gitVersion
}

// NewRepository creates a new git repository in the given directory.
func (g git) NewRepository(dst string) (Repository, error) {
	_, err := g.OpenRepository(dst)
	if err == nil {
		return nil, ErrRepoExists
	}
	cmd := exec.Exec(dst, "git", "init")
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	return &repository{
		rootDir: dst,
	}, nil
}

func (r *repository) ClearSourceControlledFilesFromWorktree() error {
	cmd := r.runCommand("git", "rm", "-r", "-q", "--ignore-unmatch", ".")
	return cmd.Err
}

func (r *repository) CommitEverythingInWorktree(message string) (Hash, error) {
	// TODO: if your build produces a .gitignore file, everything that it
	// references will be excluded from the commit. It probably shouldn't be. 😅
	cmd := r.runCommand("git", "add", ".")
	if cmd.Err != nil {
		return NilHash, cmd.Err
	}

	cmd = r.runCommand("git", "-c", "user.name='Grouse Diff'", "-c", "user.email='grouse-diff@example.com'", "commit", "--message", message, "--quiet", "--allow-empty")
	if cmd.Err != nil {
		return NilHash, cmd.Err
	}

	cmd = r.runCommand("git", "rev-parse", "--verify", "HEAD")
	if cmd.Err != nil {
		return NilHash, cmd.Err
	}
	return Hash(cmd.StdOut), nil
}
