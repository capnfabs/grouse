package pkg

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/capnfabs/grouse/internal/checkout"
	"github.com/capnfabs/grouse/internal/out"
	"github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// TODO: get rid of this, put it into a dependency context or something.
var AppFs = afero.NewOsFs()

func RunRootCommand(cmd *cobra.Command) {
	debug, err := cmd.Flags().GetBool("debug")
	check(err)
	out.Debug = debug

	context, err := parseArgs(cmd.Flags())
	if err != nil {
		out.Outln("Error:", err)
		cmd.Usage()
		os.Exit(1)
	}
	err = runMain(context)
	if err != nil {
		out.Outln("Error:", err)
		os.Exit(2)
	}
}

func commitAll(worktree *git.Worktree, msg string) (plumbing.Hash, error) {
	_, err := worktree.Add(".")
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Grouse Diff",
			Email: "grouse-diff@example.com",
			When:  time.Now(),
		},
	})
}

// If we go backwards more than this many directories looking for a git repo,
// something is very, very wrong.
const maxPathDepth = 255

// Starts at currentPath and pops directories off the stack until a git repo
// is found. Returns a git repo, the path difference between the git root and
// the directory we started in, and an error if any.
func findRepoInPath(currentPath string) (*git.Repository, string, error) {
	visitedSubdirs := ""

	for i := 0; i < maxPathDepth; i++ {
		repo, err := git.PlainOpen(currentPath)
		if err == nil {
			return repo, visitedSubdirs, nil
		} else if err == git.ErrRepositoryNotExists {
			// Try going to a parent folder
			var child string
			// Need to clean the path, because if there's a trailing slash
			// then we won't wind back a directory
			currentPath, child = path.Split(path.Clean(currentPath))
			visitedSubdirs = path.Join(child, visitedSubdirs)
			if path.Clean(currentPath) == "/" {
				// We've been unwinding the stack but we got to the root;
				// safe to assume there's no git repo.
				return nil, "", git.ErrRepositoryNotExists
			}
		} else {
			// Miscellaneous error
			return nil, "", err
		}
	}
	// We went back a very long way and still couldn't find a git repo.
	return nil, "", git.ErrRepositoryNotExists
}

func runMain(context *cmdArgs) error {
	repo, hugoRelativeRoot, err := findRepoInPath(context.repoDir)
	if err != nil {
		// Should we return these errors instead of doing this?
		return errors.WithMessagef(err, "Couldn't load the git repo in %s", context.repoDir)
	}

	refs := []checkout.ResolvedCommit{}

	for _, commit := range context.commits {
		ref, err := checkout.ResolveUserRef(repo, commit)
		if err != nil {
			return errors.WithMessagef(err, "Couldn't resolve '%s'", ref)
		}
		refs = append(refs, ref)
	}

	out.Outf("Computing diff between revisions %s and %s\n", refs[0], refs[1])

	scratchDir, err := ioutil.TempDir("", "grouse-diff")
	// If this fails, we're unable to do anything with temp storage, so just
	// panic.
	check(err)

	// Init the Output Repo
	outputDir := path.Join(scratchDir, "output")
	outputRepo, err := git.PlainInit(outputDir, false)
	// Not the user's fault and nothing we can do; panicking is ok.
	check(err)

	outputWorktree, err := outputRepo.Worktree()
	// Shouldn't be possible; because this isn't a bare repo
	check(err)

	outputHashes := []plumbing.Hash{}

	for _, ref := range refs {
		// Make sure the output directory is empty
		err = eraseDirectoryExceptRootDotGit(outputDir)
		check(err)

		srcDir := path.Join(scratchDir, "source", ref.Hash().String())
		out.Outf("Building revision %s…\n", ref)
		hash, err := process(
			outputWorktree, ref, srcDir, hugoRelativeRoot, outputDir, context.buildArgs)

		switch err.(type) {
		case *exec.ExitError:
			err := errors.Wrapf(err, "Building at commit %s failed", ref)
			return err
		case error:
			panic(err)
		}
		outputHashes = append(outputHashes, hash)
	}

	// Do the actual diff
	out.Outln("Diffing…")
	err = runDiff(outputDir, context.diffCommand, context.diffArgs, outputHashes[0], outputHashes[1])
	switch e := err.(type) {
	case *exec.ExitError:
		if strings.Contains(e.Error(), "signal: broken pipe") {
			// It's not an error; but the user exited 'less' or whatever
		} else {
			err := errors.Wrapf(
				err, "Running git %s failed", context.diffCommand)
			return err
		}
	case error:
		panic(err)
	}
	return nil
}

func eraseDirectoryExceptRootDotGit(directory string) error {
	infos, err := afero.ReadDir(AppFs, directory)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if info.Name() == ".git" {
			continue
		}

		err := AppFs.RemoveAll(path.Join(directory, info.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func process(dstWorktree *git.Worktree, ref checkout.ResolvedCommit, targetSrcDir string, hugoRelativeRoot string, outputDir string, buildArgs []string) (plumbing.Hash, error) {
	out.Debugf("Checking out %s to %s…\n", ref, targetSrcDir)
	err := checkout.ExtractCommitToDirectory(ref, targetSrcDir)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	out.Debugln("…done checking out.")

	if err = runHugo(path.Join(targetSrcDir, hugoRelativeRoot), outputDir, buildArgs); err != nil {
		return plumbing.ZeroHash, err
	}

	commitMessage := fmt.Sprintf("Website content, built from %s", ref)
	hash, err := commitAll(dstWorktree, commitMessage)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return hash, nil
}

func runHugo(hugoRootDir string, outputDir string, userArgs []string) error {
	// Put the 'destination' last. Repeated 'destination' flags only uses the
	// last one.
	// Note that we do it with the "--destination=/foo/" instead of "--destination foo"
	// because the former results in
	allArgs := append(userArgs, "--destination="+shellquote.Join(outputDir))
	cmd := exec.Command("hugo", allArgs...)
	out.Debugf("Running command\n> %s\n(from directory %s)\n", shellquote.Join(cmd.Args...), hugoRootDir)
	cmd.Dir = hugoRootDir

	// TODO: if --debug is NOT specified, should hang on to these and then only
	// print them if an error occurs.
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func runDiff(repoDir, diffCommand string, userArgs []string, hash1, hash2 plumbing.Hash) error {
	allArgs := []string{diffCommand}
	allArgs = append(allArgs, userArgs...)
	allArgs = append(allArgs, hash1.String(), hash2.String())

	cmd := exec.Command("git", allArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = repoDir
	out.Debugf("Running command %s\n", shellquote.Join(cmd.Args...))
	// This gets surfaced to the user because they're allowed to pass in diff
	// args, so it's probably (?) something they can fix?
	return cmd.Run()
}
