package pkg

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/capnfabs/grouse/internal/checkout"
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

// TODO: get rid of this, put it into the context or something.
var AppFs = afero.NewOsFs()

var rootCmd = &cobra.Command{
	Use:   "grouse [flags] <commit> [<other-commit>]",
	Short: "Diffs the output of a given Hugo git repo at different commits.",
	Long: `Diffs the output of a given Hugo git repo at different commits.

Imagine that on every commit of your Hugo site, you'd generated the site and
stored that in version control. Then, you could see exactly what's changed in
your generated site between different commits.

Grouse approximates that process.`,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		context, err := createContext(cmd.Flags())
		if err != nil {
			fmt.Println("Error:", err)
			cmd.Usage()
			os.Exit(1)
		}
		runMain(context)
	},
}

type flagSet interface {
	GetBool(string) (bool, error)
	GetString(string) (string, error)
	Args() []string
}

func createContext(flags flagSet) (*mainCmdContext, error) {
	var diffCommand string

	// Error handling in this section: parsing handles validation for all the
	// user-supplied stuff; any error here is a programming error.
	useDiffTool, err := flags.GetBool("tool")
	check(err)

	if useDiffTool {
		diffCommand = "difftool"
	} else {
		diffCommand = "diff"
	}

	diffArgsStr, err := flags.GetString("diffargs")
	check(err)

	diffArgs, err := shellquote.Split(diffArgsStr)
	if err != nil {
		return nil, errors.WithMessage(err, "Couldn't parse the value provided to --diffargs")
	}
	buildArgsStr, err := flags.GetString("buildargs")
	check(err)
	buildArgs, err := shellquote.Split(buildArgsStr)
	if err != nil {
		return nil, errors.WithMessage(err, "Couldn't parse the value provided to --buildargs")
	}

	repoDir, err := os.Getwd()
	// os.Getwd() is pretty resilient but also pretty complicated; I imagine
	// this is only something that happens if e.g. you're working in a deleted
	// directory?
	check(err)

	commits := flags.Args()
	switch len(commits) {
	case 1:
		commits = append(commits, "HEAD")
	case 2:
		// no-op
	default:
		return nil, fmt.Errorf("Requires one or two git references to diff, got %v", len(commits))
	}

	return &mainCmdContext{
		repoDir:     repoDir,
		diffCommand: diffCommand,
		commits:     commits,
		diffArgs:    diffArgs,
		buildArgs:   buildArgs,
	}, nil
}

type mainCmdContext struct {
	repoDir     string
	diffCommand string
	commits     []string
	diffArgs    []string
	buildArgs   []string
}

func Main() {
	rootCmd.Flags().String("diffargs", "", "Arguments to pass on to 'git diff'")
	rootCmd.Flags().String("buildargs", "", "Arguments to pass on to the hugo build command")
	rootCmd.Flags().BoolP("tool", "t", false, "Invoke 'git difftool' instead of 'git diff'.")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
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

func runMain(context *mainCmdContext) {
	repo, err := git.PlainOpen(context.repoDir)
	if err != nil {
		// Should we return these errors instead of doing this?
		log.Fatalf("Couldn't load the git repo in %s; is this directory a git repo?\nUnderlying error: %s", context.repoDir, err)
	}

	suppliedRef1 := context.commits[0]
	suppliedRef2 := context.commits[1]

	ref1, err := checkout.ResolveUserRef(repo, suppliedRef1)
	if err != nil {
		log.Fatalf("Couldn't resolve '%s': unknown revision\n", suppliedRef1)
	}

	ref2, err := checkout.ResolveUserRef(repo, suppliedRef2)
	if err != nil {
		log.Fatalf("Couldn't resolve '%s': unknown revision\n", suppliedRef2)
	}

	log.Printf("Computing diff between revisions %s and %s\n", ref1, ref2)
	refs := []checkout.ResolvedCommit{ref1, ref2}

	scratchDir, err := ioutil.TempDir("", "hugo_diff")
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
		hash, err := process(
			outputWorktree, ref, srcDir, outputDir, context.buildArgs)

		switch err.(type) {
		case *exec.ExitError:
			err := errors.Wrapf(err, "Building at commit %s failed", ref)
			log.Fatalf("%s\n", err)
		case error:
			panic(err)
		}
		outputHashes = append(outputHashes, hash)
	}

	// Do the actual diff
	err = runDiff(outputDir, context.diffCommand, context.diffArgs, outputHashes[0], outputHashes[1])
	switch e := err.(type) {
	case *exec.ExitError:
		if strings.Contains(e.Error(), "signal: broken pipe") {
			// It's not an error; but the user exited 'less' or whatever
		} else {
			err := errors.Wrapf(
				err, "Running git %s failed", context.diffCommand)
			log.Fatalf("%s\n", err)
		}
	case error:
		panic(err)
	}
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

func process(dstWorktree *git.Worktree, ref checkout.ResolvedCommit, hugoWorkingDir string, outputDir string, buildArgs []string) (plumbing.Hash, error) {
	log.Printf("Checking out %s to %s…\n", ref, hugoWorkingDir)
	err := checkout.ExtractCommitToDirectory(ref, hugoWorkingDir)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	log.Println("…done.")

	if err = runHugo(hugoWorkingDir, outputDir, buildArgs); err != nil {
		return plumbing.ZeroHash, err
	}

	commitMessage := fmt.Sprintf("Website content, built from %s", ref)
	hash, err := commitAll(dstWorktree, commitMessage)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return hash, nil
}

func runHugo(repoDir string, outputDir string, userArgs []string) error {
	// Put the 'destination' last. Repeated 'destination' flags only uses the
	// last one.
	// Note that we do it with the "--destination=/foo/" instead of "--destination foo"
	// because the former results in
	allArgs := append(userArgs, "--destination="+shellquote.Join(outputDir))
	cmd := exec.Command("hugo", allArgs...)
	log.Printf("Running command %s\n", shellquote.Join(cmd.Args...))
	cmd.Dir = repoDir
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
	log.Printf("Running command %s\n", shellquote.Join(cmd.Args...))
	// This gets surfaced to the user because they're allowed to pass in diff
	// args, so it's probably (?) something they can fix?
	return cmd.Run()
}
