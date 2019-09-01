package pkg

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func check(err error) {
	if err != nil {
		panic(err)
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

type commandOptions struct {
	passthruFlags []string
	commits       []string
}

func parseCommandLine() commandOptions {
	pflag.Parse()

	commits := pflag.Args()
	if len(commits) > 2 || len(commits) == 0 {
		log.Fatalf("Got %d git revisions, expected either one or two.\n", len(commits))
	}
	if len(commits) == 1 {
		commits = append(commits, "HEAD")
	}

	return commandOptions{
		commits: commits,
	}
}

func Main(diffCommand string) {
	opts := parseCommandLine()

	repoDir, err := os.Getwd()
	check(err)
	repo, err := git.PlainOpen(repoDir)
	check(err)

	suppliedRef1 := opts.commits[0]
	suppliedRef2 := opts.commits[1]

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
	eraseDirectoryExceptDotGit(outputDir)

	// Run Hugo for the second commit
	commit2Dir := path.Join(scratchDir, "source_ref2")
	hash2 := process(repo, outputRepo, ref2, commit2Dir, outputDir)

	// Do the actual diff
	runDiff(outputDir, diffCommand, hash1, hash2)
}

func eraseDirectoryExceptDotGit(directory string) {
	infos, err := ioutil.ReadDir(directory)
	check(err)
	for _, info := range infos {
		if info.Name() == ".git" {
			continue
		}

		err := os.RemoveAll(path.Join(directory, info.Name()))
		check(err)
	}
}

func process(srcRepo *git.Repository, dstRepo *git.Repository, ref *resolvedUserRef, hugoWorkingDir string, outputDir string) plumbing.Hash {
	commit, err := srcRepo.CommitObject(*ref.resolvedRef)
	check(err)
	files, err := commit.Files()
	check(err)

	log.Printf("Checking out %s to %s...\n", ref, hugoWorkingDir)
	copyFilesToDir(files, hugoWorkingDir)
	log.Println("...done.")

	runHugo(hugoWorkingDir, outputDir)

	// commit and return hash
	worktree, err := dstRepo.Worktree()
	check(err)
	commitMessage := fmt.Sprintf("Website content, built from %s", ref)
	hash, err := commitAll(worktree, commitMessage)
	check(err)
	return hash
}

func copyFilesToDir(files *object.FileIter, targetDir string) error {
	return files.ForEach(func(file *object.File) error {
		outputPath := path.Join(targetDir, file.Name)
		os.MkdirAll(path.Dir(outputPath), os.ModeDir|0700)

		outputFile, err := os.Create(outputPath)
		check(err)

		reader, err := file.Reader()
		check(err)

		_, err = io.Copy(outputFile, reader)
		check(err)

		return nil
	})
}

func runHugo(repoDir string, outputDir string) {
	cmd := exec.Command("hugo", "--destination", outputDir)
	// TODO: this will print the wrong thing if any args have spaces in them.
	// Use a library for this instead
	log.Printf("Running command %s\n", strings.Join(cmd.Args, " "))
	cmd.Dir = repoDir
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	check(err)
}

func runDiff(repoDir, diffCommand string, hash1, hash2 plumbing.Hash) {
	cmd := exec.Command("git", diffCommand, hash1.String(), hash2.String())
	// TODO: this will print the wrong thing if any args have spaces in them.
	// Use a library for this instead
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = repoDir
	log.Printf("Running command %s\n", strings.Join(cmd.Args, " "))
	cmd.Run()
}
