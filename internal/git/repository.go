package git

import (
	"os"
	"path"
	"strings"

	"github.com/capnfabs/grouse/internal/exec"
	"github.com/capnfabs/grouse/internal/out"
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

type submodInfo struct {
	// (e.g. `submodules.themes/paperesque`)
	configPrefix string
	path         string
}

func (r *repository) getSubmodulePaths() ([]submodInfo, error) {
	_, err := os.Lstat(path.Join(r.rootDir, ".gitmodules"))
	if err != nil && os.IsNotExist(err) {
		// No submodules, just chill.
		out.Debugln("No submodules found in repo.")
		return []submodInfo{}, nil
	} else if err != nil {
		// I can't imagine what error this could be...
		out.Debugf("Got error %v when attempting to find .gitmodules file; pretending there isn't any\n", err)
		return []submodInfo{}, err
	}
	// This fetches one line per submodule, e.g.
	// submodule.themes/paperesque.path themes/paperesque
	cmd := r.runCommand("git", "config", "--file", ".gitmodules", "--get-regexp", `submodule\..*\.path`)
	if cmd.Err != nil {
		out.Debugf("Got error %v when attempting to load submodule config; pretending there aren't any submodules\n", err)
		return []submodInfo{}, cmd.Err
	}
	lines := strings.Split(cmd.StdOut, "\n")
	submodules := []submodInfo{}
	for _, line := range lines {
		if line != "" {
			fields := strings.Fields(line)
			submodules = append(submodules, submodInfo{
				configPrefix: strings.TrimSuffix(fields[0], ".path"),
				path:         fields[1],
			})
		}
	}
	return submodules, nil
}

// prepSubmodulesForWorktree is Dark Magic that bootstraps git submodules for a
// secondary worktree so that you don't need to have internet to run
// `submodule update` within the worktree.
func prepSubmodulesForSharedClone(src *repository, dst *worktreeRepository) error {
	submodPaths, err := src.getSubmodulePaths()
	if err != nil {
		return err
	}

	cmd := dst.runCommand("git", "submodule", "init")
	if cmd.Err != nil {
		// TODO: better error handling
		// Dunno what to make of this
		return cmd.Err
	}

	for _, submod := range submodPaths {
		submodRepo, err := src.gitInterface.openRepository(path.Join(src.rootDir, submod.path))

		if err != nil {
			// Something mysterious went wrong; just ignore and continue.
			out.Debugf("Unable to prepare submodule: %v\n", err)
			continue
		}

		if submodRepo.rootDir == src.rootDir {
			// TODO: abstract this error handling better, by e.g. making
			// openRepository take an argument as to whether it's allowed to look further up the tree.

			// This indicates that the submodule isn't in the right place etc etc.
			out.Debugf("Thought %s was a repo, but it wasn't, abandoning attempt to load submodule\n", submod.path)
			continue
		}

		out.Debugf("Recursively cloning submodule at %s...\n", submod.path)
		clonedSubmodRepo, err := submodRepo.recursiveSharedCloneTo(path.Join(dst.rootDir, submod.path))

		if err != nil {
			// this should only happen in real bad circumstances, so panic,
			// even though it's probably recoverable.
			panic(err)
		}

		// TODO: extract this code to patch up the config
		urlConfigName := submod.configPrefix + ".url"
		cmd = src.runCommand("git", "config", urlConfigName)
		realRemoteURL := cmd.StdOut
		if cmd.Err != nil {
			panic(cmd.Err)
		}
		cmd = dst.runCommand("git", "config", urlConfigName, realRemoteURL)
		if cmd.Err != nil {
			panic(cmd.Err)
		}
		// _AND_ we have to change the remote so we can pull commits if we have to.
		cmd = clonedSubmodRepo.runCommand("git", "remote", "set-url", "origin", realRemoteURL)
		if cmd.Err != nil {
			panic(cmd.Err)
		}
	}
	return nil
}

func (r *repository) RecursiveSharedCloneTo(dst string) (WorktreeRepository, error) {
	return r.recursiveSharedCloneTo(dst)
}

func (r *repository) recursiveSharedCloneTo(dst string) (*worktreeRepository, error) {
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
	// --force is here because of an edge-case -- if you had submodules, and
	// then replace them with the actual content in the git repo, you need to
	// use --force to tell git all the old submodule content.
	// NOTE: I ran into one case where the gitmodules file couldn't be removed
	// when attempting this on something with nested submodules that had been
	// moved into the root repo. You can test this out with
	// `test-fixtures/unirepo-gitinfo.zip` between commits "742de0a", "353bfcb".
	cmd := w.runCommand("git", "checkout", "--force", "--detach", string(commit.Hash()))
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

	// Checkout submodules.
	// --recursive -- automatically do everything in the entire tree
	// --init -- if there are uninitialized submodules, then init them.
	cmd = w.runCommand("git", "submodule", "update", "--recursive", "--init")
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
