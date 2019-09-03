package pkg

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

var GitCommand = "diff"

var DiffArgs string
var BuildArgs string

var AppFs = afero.NewOsFs()

var rootCmd = &cobra.Command{
	Use:   "hugo-diff[tool] [flags] <commit> [<other-commit>]",
	Short: "Diffs the output of a given Hugo git repo at different commits.",
	Long: `Diffs the output of a given Hugo git repo at different commits.

Imagine that on every commit of your Hugo site, you'd generated the site and
stored that in version control. Then, you could see exactly what's changed in
your generated site between different commits.

hugo-diff approximates that process.`,
	DisableFlagsInUseLine: true,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || len(args) > 2 {
			return fmt.Errorf("Requires one or two git references to diff, got %v", len(args))
		}

		// TODO: resolve git SHA1s here?

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 {
			args = append(args, "HEAD")
		}

		if GitCommand != "diff" && GitCommand != "difftool" {
			panic("Unexpected git command")
		}
		runMain(GitCommand, args)
	},
}

func Main() {
	rootCmd.Flags().StringVar(&DiffArgs, "diff-args", "", "Arguments to pass on to 'git diff'")
	rootCmd.Flags().StringVar(&BuildArgs, "build-args", "", "Arguments to pass on to the hugo build command")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type resolvedUserRef struct {
	userRef     string
	repo        *git.Repository
	resolvedRef *plumbing.Hash
}

func resolveRef(repo *git.Repository, userRef string) (*resolvedUserRef, error) {
	ref, err := repo.ResolveRevision(plumbing.Revision(userRef))
	if err != nil {
		return nil, err
	}

	return &resolvedUserRef{
		userRef:     userRef,
		repo:        repo,
		resolvedRef: ref,
	}, nil
}

func (r *resolvedUserRef) String() string {
	return fmt.Sprintf("%s (%s)", r.userRef, r.resolvedRef.String()[:7])
}

func commitAll(worktree *git.Worktree, msg string) (plumbing.Hash, error) {
	_, err := worktree.Add(".")
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Hugo Diff (hugo-diff)",
			Email: "hugo-diff@capnfabs.net",
			When:  time.Now(),
		},
	})
}

func runMain(diffCommand string, commits []string) {
	repoDir, err := os.Getwd()
	check(err)
	repo, err := git.PlainOpen(repoDir)
	check(err)

	suppliedRef1 := commits[0]
	suppliedRef2 := commits[1]

	ref1, err := resolveRef(repo, suppliedRef1)
	if err != nil {
		log.Fatalf("Couldn't resolve '%s': unknown revision\n", suppliedRef1)
	}
	ref2, err := resolveRef(repo, suppliedRef2)
	if err != nil {
		log.Fatalf("Couldn't resolve '%s': unknown revision\n", suppliedRef2)
	}

	log.Printf("Computing diff between revisions %s and %s\n", ref1, ref2)

	scratchDir, err := ioutil.TempDir("", "hugo_diff")
	check(err)

	// Init the Output Repo
	outputDir := path.Join(scratchDir, "output")
	outputRepo, err := git.PlainInit(outputDir, false)
	check(err)

	// Run Hugo for the first commit
	commit1Dir := path.Join(scratchDir, "source_ref1")
	hash1 := process(repo, outputRepo, ref1, commit1Dir, outputDir)

	// Now erase the directory
	eraseDirectoryExceptRootDotGit(outputDir)

	// Run Hugo for the second commit
	commit2Dir := path.Join(scratchDir, "source_ref2")
	hash2 := process(repo, outputRepo, ref2, commit2Dir, outputDir)

	// Do the actual diff
	runDiff(outputDir, diffCommand, hash1, hash2)
}

func eraseDirectoryExceptRootDotGit(directory string) {
	fmt.Println("appfs", AppFs)
	infos, err := afero.ReadDir(AppFs, directory)
	check(err)
	for _, info := range infos {
		if info.Name() == ".git" {
			continue
		}

		err := AppFs.RemoveAll(path.Join(directory, info.Name()))
		check(err)
	}
}

func process(srcRepo *git.Repository, dstRepo *git.Repository, ref *resolvedUserRef, hugoWorkingDir string, outputDir string) plumbing.Hash {
	commit, err := srcRepo.CommitObject(*ref.resolvedRef)
	check(err)
	log.Printf("Checking out %s to %s…\n", ref, hugoWorkingDir)
	wt, err := srcRepo.Worktree()
	check(err)
	extractFilesAtCommitToDir(wt, srcRepo, commit, hugoWorkingDir)
	log.Println("…done.")

	runHugo(hugoWorkingDir, outputDir)

	// commit and return hash
	worktree, err := dstRepo.Worktree()
	check(err)
	commitMessage := fmt.Sprintf("Website content, built from %s", ref)
	hash, err := commitAll(worktree, commitMessage)
	check(err)
	return hash
}

func extractFilesAtCommitToDir(
	worktree *git.Worktree,
	repo *git.Repository,
	commit *object.Commit, targetDir string) error {
	files, err := commit.Files()
	check(err)
	err = files.ForEach(func(file *object.File) error {
		outputPath := path.Join(targetDir, file.Name)
		fmt.Println("file", file.Name)
		AppFs.MkdirAll(path.Dir(outputPath), os.ModeDir|0700)

		outputFile, err := AppFs.Create(outputPath)
		check(err)

		reader, err := file.Reader()
		check(err)

		_, err = io.Copy(outputFile, reader)
		check(err)

		return nil
	})
	if err != nil {
		return err
	}
	file, err := commit.File(".gitmodules")
	fmt.Println("Error for gitmodules", err)
	if err == nil {
		// Submodules in use at this commit.
		fmt.Println("Found submodules file!")
		f, err := file.Reader()
		check(err)

		defer f.Close()
		input, err := ioutil.ReadAll(f)
		check(err)
		m := config.NewModules()
		err = m.Unmarshal(input)
		check(err)
		for _, v := range m.Submodules {
			// Get the commit hash
			tree, _ := commit.Tree()
			// Need to check this error
			entry, _ := tree.FindEntry(v.Path)
			commitRef := entry.Hash
			fmt.Println("commit-loaded entry", entry)

			fmt.Println(v, "=>", commitRef)
			// YIKES this is going to be intense.
			// (1) try and load the repo from the _current_ worktree and see if the commit SHA from this one is floating around there. If it is, use that. There's a pretty good chance of that IMO.
			// (2) try and clone the repo into memory or a cache or something. Probs a cache, because people might run this multiple times.
			// The return type, for both approaches, will be an objects.Commit.
			resolveSubmodule(v, worktree)
		}
	}
	return nil
}

func resolveSubmodule(submodule *config.Submodule, worktree *git.Worktree) {

}

func runHugo(repoDir string, outputDir string) {
	args, err := shellquote.Split(BuildArgs)
	check(err)
	// Put the 'destination' last. Repeated 'destination' flags only uses the
	// last one.
	allArgs := append(args, "--destination", outputDir)
	cmd := exec.Command("hugo", allArgs...)
	log.Printf("Running command %s\n", shellquote.Join(cmd.Args...))
	cmd.Dir = repoDir
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	check(err)
}

func runDiff(repoDir, diffCommand string, hash1, hash2 plumbing.Hash) {
	args, err := shellquote.Split(DiffArgs)
	check(err)
	allArgs := []string{diffCommand}
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, hash1.String(), hash2.String())

	cmd := exec.Command("git", allArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = repoDir
	log.Printf("Running command %s\n", shellquote.Join(cmd.Args...))
	// TODO: add rescue in case of failure.
	cmd.Run()
}
