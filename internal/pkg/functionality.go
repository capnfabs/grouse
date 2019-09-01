package pkg

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func ok(err error) {
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

func Main(diffCommand string) {
	flag.Parse()
	args := flag.Args()
	if len(args) > 2 || len(args) == 0 {
		log.Fatalf("Got %d git revisions, expected either one or two.\n", len(args))
	}
	repoDir, _ := os.Getwd()
	repo, _ := git.PlainOpen(repoDir)

	suppliedRef1 := args[0]
	var suppliedRef2 string

	if len(args) == 1 {
		suppliedRef2 = "HEAD"
	} else {
		suppliedRef2 = args[1]
	}

	ref1, err := resolveRef(repo, suppliedRef1)
	// TODO: we're actually expecting this to happen semi-regularly so we
	// should handle it elegantly.
	ok(err)
	ref2, err := resolveRef(repo, suppliedRef2)
	ok(err)

	log.Printf("Computing diff between revisions %s and %s\n", ref1, ref2)

	scratchDir, err := ioutil.TempDir("", "hugo_diff")
	ok(err)

	commit1Dir := path.Join(scratchDir, "source_ref1")
	outputDir := path.Join(scratchDir, "output")

	// Init the Output Repo
	outputRepo, err := git.PlainInit(outputDir, false)
	ok(err)
	worktree, err := outputRepo.Worktree()
	ok(err)

	process(repo, ref1, commit1Dir, outputDir)

	commitMessage := fmt.Sprintf("Website content, built from %s", ref1)
	hash1, err := commitAll(worktree, commitMessage)
	ok(err)

	// Now erase the directory
	infos, err := ioutil.ReadDir(outputDir)
	ok(err)
	for _, info := range infos {
		if info.Name() == ".git" {
			continue
		}

		err := os.RemoveAll(path.Join(outputDir, info.Name()))
		ok(err)
	}

	// Alright let's do the second checkout.
	commit2Dir := path.Join(scratchDir, "source_ref2")
	process(repo, ref2, commit2Dir, outputDir)

	commitMessage = fmt.Sprintf("Website content, built from %s", ref2)
	hash2, err := commitAll(worktree, commitMessage)
	ok(err)

	runDiff(outputDir, diffCommand, hash1, hash2)
}

func process(repo *git.Repository, ref *resolvedUserRef, srcDir string, outputDir string) {
	commit, err := repo.CommitObject(*ref.resolvedRef)
	ok(err)
	files, err := commit.Files()
	ok(err)

	log.Printf("Checking out %s to %s\n", ref, srcDir)
	copyFilesToDir(files, srcDir)
	log.Println("Done.")

	runHugo(srcDir, outputDir)
}

func copyFilesToDir(files *object.FileIter, targetDir string) error {
	return files.ForEach(func(file *object.File) error {
		outputPath := path.Join(targetDir, file.Name)
		os.MkdirAll(path.Dir(outputPath), os.ModeDir|0700)

		outputFile, err := os.Create(outputPath)
		ok(err)

		reader, err := file.Reader()
		ok(err)

		_, err = io.Copy(outputFile, reader)
		ok(err)

		return nil
	})
}

func runHugo(repoDir string, outputDir string) {
	cmd := exec.Command("hugo", "--destination", outputDir)
	log.Printf("Running command %s\n", strings.Join(cmd.Args, " "))
	cmd.Dir = repoDir
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	ok(err)
}

func runDiff(repoDir, diffCommand string, hash1, hash2 plumbing.Hash) {
	cmd := exec.Command("git", diffCommand, hash1.String(), hash2.String())
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = repoDir
	log.Printf("Running command %s\n", strings.Join(cmd.Args, " "))
	cmd.Run()
}
