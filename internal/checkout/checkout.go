package checkout

import (
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/capnfabs/grouse/internal/out"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

func copyFileTo(fs afero.Fs, file *object.File, outputPath string) error {
	fs.MkdirAll(path.Dir(outputPath), os.ModeDir|0700)

	outputFile, err := fs.Create(outputPath)
	if err != nil {
		return err
	}

	reader, err := file.Reader()
	if err != nil {
		return err
	}

	_, err = io.Copy(outputFile, reader)
	return err
}

func parseModulesFile(file *object.File) (*config.Modules, error) {
	f, err := file.Reader()
	if err != nil {
		return nil, err
	}

	defer f.Close()
	input, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	m := config.NewModules()
	err = m.Unmarshal(input)

	if err != nil {
		return nil, err
	}
	return m, nil
}

func tryCopyingSubmodules(context *extractCommitContext, ref ResolvedCommit, targetDir string) error {
	file, err := ref.Commit().File(".gitmodules")
	if err != nil {
		// Oh, ok, no .gitmodules file, but this isn't considered to be an error
		// for this function.
		return nil
	}

	// Submodules in use at this commit.
	// TODO: handle nested submodules. I've got a hunch that our own submodule
	// handling will play badly with go-git's.
	out.Debugln("Found submodules file, figuring out where to source the submodule contents from")

	m, err := parseModulesFile(file)
	if err != nil {
		return err
	}

	for _, v := range m.Submodules {

		// Get the commit hash for this submodule
		tree, _ := ref.Commit().Tree()
		entry, err := tree.FindEntry(v.Path)
		if err != nil {
			return err
		}
		submoduleRef, err := resolveSubmodule(context, v, entry.Hash, ref.Worktree())
		if err != nil {
			return errors.Wrap(err, "Couldn't load submodule")
		}
		subModuleOutputPath := path.Join(targetDir, v.Path)
		err = extractCommitToDirectoryWithContext(context, submoduleRef, subModuleOutputPath)
		if err != nil {
			return errors.Wrap(err, "Couldn't write submodule contents to filesystem")
		}
	}
	return nil
}

type extractCommitContext struct {
	fs              afero.Fs
	submoduleCloner func(s storage.Storer, worktree billy.Filesystem, o *git.CloneOptions) (*git.Repository, error)
}

var defaultContext = extractCommitContext{
	fs:              afero.NewOsFs(),
	submoduleCloner: git.Clone,
}

func extractCommitToDirectoryWithContext(context *extractCommitContext, ref ResolvedCommit, outputDirectory string) error {
	files, err := ref.Commit().Files()
	if err != nil {
		return err
	}
	err = files.ForEach(func(file *object.File) error {
		outputPath := path.Join(outputDirectory, file.Name)
		return copyFileTo(context.fs, file, outputPath)
	})
	if err != nil {
		return err
	}
	return tryCopyingSubmodules(context, ref, outputDirectory)
}

func ExtractCommitToDirectory(ref ResolvedCommit, outputDirectory string) error {
	return extractCommitToDirectoryWithContext(&defaultContext, ref, outputDirectory)
}

func loadSubmoduleFromCurrentWorktree(
	submodule *config.Submodule,
	commitRef plumbing.Hash,
	worktree *git.Worktree) (ResolvedCommit, error) {

	sub, err := worktree.Submodule(submodule.Name)
	if err != nil {
		return nil, err
	}
	repo, err := sub.Repository()
	if err != nil {
		return nil, err
	}
	return resolveHash(repo, commitRef)
}

func loadSubmoduleFromRemote(
	context *extractCommitContext,
	submodule *config.Submodule,
	commitRef plumbing.Hash) (ResolvedCommit, error) {
	// It's kinda weird that this writes to the terminal but it's worth the user knowing that we're
	// attempting to hit a remote repo
	out.Outln("Couldn't find submodule commit in worktree, cloning from remote…")

	fs := memfs.New()
	storer := memory.NewStorage()

	// NOTE: it would be _super_ nice to do the following here:
	// - Attempt to fetch just the given commit (some servers support this, some don't; see https://stackoverflow.com/a/3489576/996592)
	// - Attempt to fetch just the given _branch_ being tracked, look for the commit in the history there
	// - Fetch the whole repo, checkout the commit.
	// - Cache the whole thing so it's faster next time.
	// Designing the cache to work with this feels like it's going to be complicated; so I'm skipping it for v1.
	// ESPECIALLY because we're expecting the "load from worktree" strategy to work pretty regularly.
	repo, err := context.submoduleCloner(storer, fs, &git.CloneOptions{
		URL: submodule.URL,
	})
	if err != nil {
		return nil, err
	}
	out.Outln("…done cloning from remote.")
	return resolveHash(repo, commitRef)
}

// Here's the strategy
// (1) try and load the repo from the _current_ worktree and see if the commit SHA from this one is floating around there. If it is, use that. This will hopefully be 90% of cases.
// (2) try and clone the repo into memory or a cache. It'd be really good to optimise this (see the docs in loadSubmoduleForRemote, but again, this will be less frequent so it can wait for 1.1)
func resolveSubmodule(context *extractCommitContext, submodule *config.Submodule, commitRef plumbing.Hash, worktree *git.Worktree) (ResolvedCommit, error) {
	if r, err := loadSubmoduleFromCurrentWorktree(submodule, commitRef, worktree); err == nil {
		out.Debugln("Found commit for submodule in worktree.")
		return r, nil
	}

	r, err := loadSubmoduleFromRemote(context, submodule, commitRef)
	return r, err
}
