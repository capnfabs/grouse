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

	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

var DiffArgs string
var BuildArgs string
var UseGitDiffTool bool

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

		var gitCommand string
		if UseGitDiffTool {
			gitCommand = "difftool"
		} else {
			gitCommand = "diff"
		}
		runMain(gitCommand, args)
	},
}

func Main() {
	rootCmd.Flags().StringVar(&DiffArgs, "diff-args", "", "Arguments to pass on to 'git diff'")
	rootCmd.Flags().StringVar(&BuildArgs, "build-args", "", "Arguments to pass on to the hugo build command")
	rootCmd.Flags().BoolVarP(&UseGitDiffTool, "tool", "t", false, "Invoke 'git difftool' instead of 'git diff'.")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type resolvedUserRef struct {
	userRef string
	resolvedHash
}

type resolvedHash struct {
	repo     *git.Repository
	hash     plumbing.Hash
	commit   *object.Commit
	worktree *git.Worktree
}

type resolved interface {
	Repo() *git.Repository
	Hash() plumbing.Hash
	Commit() *object.Commit
	Worktree() *git.Worktree
}

func (r *resolvedHash) Repo() *git.Repository {
	return r.repo
}
func (r *resolvedHash) Hash() plumbing.Hash {
	return r.hash
}
func (r *resolvedHash) Commit() *object.Commit {
	return r.commit
}

func (r *resolvedHash) Worktree() *git.Worktree {
	return r.worktree
}

func resolveUserRef(repo *git.Repository, userRef string) (*resolvedUserRef, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(userRef))
	if err != nil {
		return nil, err
	}

	resHash, err := resolveHash(repo, *hash)
	if err != nil {
		return nil, err
	}
	return &resolvedUserRef{
		userRef:      userRef,
		resolvedHash: *resHash,
	}, nil

}

func resolveHash(repo *git.Repository, hash plumbing.Hash) (*resolvedHash, error) {
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, err
	}

	return &resolvedHash{
		repo:     repo,
		hash:     hash,
		commit:   commit,
		worktree: worktree,
	}, nil
}

func (r *resolvedUserRef) String() string {
	return fmt.Sprintf("%s (%s)", r.userRef, r.hash.String()[:7])
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

func runMain(diffCommand string, commits []string) {
	repoDir, err := os.Getwd()
	check(err)
	repo, err := git.PlainOpen(repoDir)
	check(err)

	suppliedRef1 := commits[0]
	suppliedRef2 := commits[1]

	ref1, err := resolveUserRef(repo, suppliedRef1)
	if err != nil {
		log.Fatalf("Couldn't resolve '%s': unknown revision\n", suppliedRef1)
	}
	ref2, err := resolveUserRef(repo, suppliedRef2)
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
	hash1 := process(outputRepo, ref1, commit1Dir, outputDir)

	// Now erase the directory
	eraseDirectoryExceptRootDotGit(outputDir)

	// Run Hugo for the second commit
	commit2Dir := path.Join(scratchDir, "source_ref2")
	hash2 := process(outputRepo, ref2, commit2Dir, outputDir)

	// Do the actual diff
	runDiff(outputDir, diffCommand, hash1, hash2)
}

func eraseDirectoryExceptRootDotGit(directory string) {
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

func process(dstRepo *git.Repository, ref resolved, hugoWorkingDir string, outputDir string) plumbing.Hash {
	err := extractCommitToDirectory(ref, hugoWorkingDir)
	check(err)

	runHugo(hugoWorkingDir, outputDir)

	// commit and return hash
	worktree, err := dstRepo.Worktree()
	check(err)
	commitMessage := fmt.Sprintf("Website content, built from %s", ref)
	hash, err := commitAll(worktree, commitMessage)
	check(err)
	return hash
}

func copyFileTo(file *object.File, outputPath string) error {
	AppFs.MkdirAll(path.Dir(outputPath), os.ModeDir|0700)

	outputFile, err := AppFs.Create(outputPath)
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

func tryCopyingSubmodules(ref resolved, targetDir string) error {
	file, err := ref.Commit().File(".gitmodules")
	if err == nil {
		// Submodules in use at this commit.
		// TODO: handle nested submodules. I've got a hunch that our own submodule
		// handling will play badly with go-git's.
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

			// Get the commit hash for this submodule
			tree, _ := ref.Commit().Tree()
			entry, err := tree.FindEntry(v.Path)
			if err != nil {
				return err
			}
			submoduleRef, err := resolveSubmodule(v, entry.Hash, ref.Worktree())
			if err != nil {
				fmt.Println("Couldn't load submodule: ", err)
				return err
			}
			subModuleOutputPath := path.Join(targetDir, v.Path)
			err = extractCommitToDirectory(submoduleRef, subModuleOutputPath)
			if err != nil {
				fmt.Println("Couldn't load submodule to filesystem:", err)
				return err
			}
		}
	}
	return nil
}

func extractCommitToDirectory(ref resolved, outputDirectory string) error {
	// TODO: don't do this for submodules.
	log.Printf("Checking out %s to %s…\n", ref, outputDirectory)
	defer log.Println("…done.")

	files, err := ref.Commit().Files()
	if err != nil {
		return err
	}
	err = files.ForEach(func(file *object.File) error {
		outputPath := path.Join(outputDirectory, file.Name)
		return copyFileTo(file, outputPath)
	})
	if err != nil {
		return err
	}
	return tryCopyingSubmodules(ref, outputDirectory)
}

func loadSubmoduleFromCurrentWorktree(
	submodule *config.Submodule,
	commitRef plumbing.Hash,
	worktree *git.Worktree) (resolved, error) {

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
	submodule *config.Submodule,
	commitRef plumbing.Hash) (resolved, error) {

	fs := memfs.New()
	storer := memory.NewStorage()

	// NOTE: it would be _super_ nice to do the following here:
	// - Attempt to fetch just the given commit (some servers support this, some don't; see https://stackoverflow.com/a/3489576/996592)
	// - Attempt to fetch just the given _branch_ being tracked, look for the commit in the history there
	// - Fetch the whole repo, checkout the commit.
	// - Cache the whole thing so it's faster next time.
	// Designing the cache to work with this feels like it's going to be complicated; so I'm skipping it for v1.
	// ESPECIALLY because we're expecting the "load from worktree" strategy to work pretty regularly.
	repo, err := git.Clone(storer, fs, &git.CloneOptions{
		URL: submodule.URL,
	})
	if err != nil {
		return nil, err
	}
	return resolveHash(repo, commitRef)
}

// Here's the strategy
// (1) try and load the repo from the _current_ worktree and see if the commit SHA from this one is floating around there. If it is, use that. This will hopefully be 90% of cases.
// (2) try and clone the repo into memory or a cache. It'd be really good to optimise this (see the docs in loadSubmoduleForRemote, but again, this will be less frequent so it can wait for 1.1)
func resolveSubmodule(submodule *config.Submodule, commitRef plumbing.Hash, worktree *git.Worktree) (resolved, error) {
	if r, err := loadSubmoduleFromCurrentWorktree(submodule, commitRef, worktree); err == nil {
		fmt.Println("Found commit for submodule in worktree.")
		return r, nil
	}

	fmt.Println("Couldn't find submodule commit in worktree, falling back to clone strategy")
	r, err := loadSubmoduleFromRemote(submodule, commitRef)
	return r, err
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
