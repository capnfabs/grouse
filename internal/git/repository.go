package git

import (
	"os"
	"path"
	"strings"

	"github.com/capnfabs/grouse/internal/exec"
)

// Repository represents a git repository somewhere on disk. It's not able to
// perform destructive actions -- for that, you need to obtain a WriteableRepository or a WorkingRepository.
type Repository interface {
	RootDir() string
	ResolveCommit(ref string) (ResolvedUserRef, error)
	RecursiveSharedCloneTo(dst string) (WorktreeRepository, error)
}

// concrete implementation
type repository struct {
	rootDir      string
	gitInterface *git
}

// WorktreeRepository represents a temporary scratch git repository where it's
// ok to checkout files, and delete the repository later, but not to commit
// things.
type WorktreeRepository interface {
	Repository
	Remove() error
	Checkout(commit ResolvedCommit) error
}

type worktreeRepository struct {
	repository
}

// WriteableRepository is a Repository that allows commits / edits to the git
// history.
type WriteableRepository interface {
	// WriteableRepository is intentionally not a superset of WorktreeRepository
	// so we don't have to be aware of Checkouts etc in the Commit / Clear
	// methods.
	Repository
	CommitEverythingInWorktree(message string) (Hash, error)
	ClearSourceControlledFilesFromWorktree() error
}

type writeableRepo struct {
	repository
}

func (r *repository) RootDir() string {
	return r.rootDir
}

func (r *repository) runCommand(args ...string) exec.CmdResult {
	return exec.Exec(r.rootDir, args...)
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

func (r *repository) getSubmodulePaths() ([]string, error) {
	_, err := os.Lstat(path.Join(r.rootDir, ".gitmodules"))
	if err != nil && os.IsNotExist(err) {
		// No submodules, just chill.
		// TODO: make this a log statement
		println("NO SUBMODULES")
		return []string{}, nil
	} else if err != nil {
		// I can't imagine what error this could be...
		// TODO: log this.
		println(err)
		return []string{}, err
	}
	// This fetches one line per submodule, e.g.
	// submodule.themes/paperesque.path themes/paperesque
	cmd := r.runCommand("git", "config", "--file", ".gitmodules", "--get-regexp", `submodule\..*\.path`)
	if cmd.Err != nil {
		// TODO: Log this
		println("COMMAND ERROR")
		return []string{}, cmd.Err
	}
	lines := strings.Split(cmd.StdOut, "\n")
	submodulePaths := []string{}
	for _, line := range lines {
		if line != "" {
			submodulePaths = append(submodulePaths, strings.Fields(line)[1])
		}
	}
	return submodulePaths, nil
}

// prepSubmodulesForWorktree is Dark Magic that bootstraps git submodules for a
// secondary worktree so that you don't need to have internet to run
// `submodule update` within the worktree.
func prepSubmodulesForSharedClone(src *repository, dst *worktreeRepository) error {
	submodPaths, err := src.getSubmodulePaths()
	if err != nil {
		return err
	}

	for _, p := range submodPaths {
		println(src.rootDir, p, src.gitInterface)
		submodRepo, err := src.gitInterface.OpenRepository(path.Join(src.rootDir, p))
		if err != nil {
			// maybe it wasn't checked out, not important, just continue.
			// TODO: log the error
			continue
		}
		_, err = submodRepo.RecursiveSharedCloneTo(path.Join(dst.rootDir, p))

		if err != nil {
			// this should only happen in real bad circumstances, so panic,
			// even though it's probably recoverable.
			panic(err)
		}
	}
	return nil
}

func (r *repository) RecursiveSharedCloneTo(dst string) (WorktreeRepository, error) {
	var args []string

	// Note: using "--no-checkout" here works great for the root repo, but
	// breaks things if you use it on submodule repos, because when you run
	// `git submodule update` later, it refuses to clobber the dirty state in
	// the submodule.
	args = []string{"git", "clone", "--shared", r.rootDir, dst}

	cmd := r.runCommand(args...)
	if cmd.Err != nil {
		return nil, cmd.Err
	}

	wt := &worktreeRepository{
		repository: repository{rootDir: dst, gitInterface: r.gitInterface},
	}

	if err := prepSubmodulesForSharedClone(r, wt); err != nil {
		return nil, err
	}
	return wt, nil
}

func (w *worktreeRepository) Checkout(commit ResolvedCommit) error {
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

func (w *worktreeRepository) Remove() error {
	return os.RemoveAll(w.rootDir)
}

func (r *writeableRepo) ClearSourceControlledFilesFromWorktree() error {
	// TODO: document switches.
	cmd := r.runCommand("git", "rm", "-r", "-q", "--ignore-unmatch", ".")
	return cmd.Err
}

func (r *writeableRepo) CommitEverythingInWorktree(message string) (Hash, error) {
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
