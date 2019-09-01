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

	ref1, _ := repo.ResolveRevision(plumbing.Revision(suppliedRef1))
	ref2, _ := repo.ResolveRevision(plumbing.Revision(suppliedRef2))

	log.Printf("Computing diff between revisions %s (%s) and %s (%s)\n", suppliedRef1, ref1, suppliedRef2, ref2)

	scratchDir, err := ioutil.TempDir("", "hugo_diff")
	ok(err)

	commit1Dir := path.Join(scratchDir, "source_ref1")
	outputDir := path.Join(scratchDir, "output")

	// Init Repo
	outputRepo, err := git.PlainInit(outputDir, false)
	ok(err)
	worktree, err := outputRepo.Worktree()
	ok(err)

	process(repo, ref1, commit1Dir, outputDir)

	_, err = worktree.Add(".")
	ok(err)
	hash1, err := worktree.Commit(fmt.Sprintf("Website content, built from %s (%s)", suppliedRef1, ref1), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Hugo Diff (hugo-diff)",
			Email: "hugo-diff@capnfabs.net",
			When:  time.Now(),
		},
	})
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

	commit2Dir := path.Join(scratchDir, "source_ref2")
	process(repo, ref2, commit2Dir, outputDir)

	_, err = worktree.Add(".")
	ok(err)
	hash2, err := worktree.Commit(fmt.Sprintf("Website content, built from %s (%s)", suppliedRef2, ref2), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Hugo Diff (hugo-diff)",
			Email: "hugo-diff@capnfabs.net",
			When:  time.Now(),
		},
	})
	ok(err)

	runDiff(outputDir, diffCommand, hash1, hash2)
}

func process(repo *git.Repository, ref *plumbing.Hash, srcDir string, outputDir string) {
	commit, err := repo.CommitObject(*ref)
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
