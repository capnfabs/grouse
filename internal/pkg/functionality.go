package pkg

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

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
	command1OutputDir := path.Join(scratchDir, "output_ref1")

	process(repo, ref1, commit1Dir, command1OutputDir)

	commit2Dir := path.Join(scratchDir, "source_ref2")
	command2OutputDir := path.Join(scratchDir, "output_ref2")
	process(repo, ref2, commit2Dir, command2OutputDir)

	runDiff(diffCommand, command1OutputDir, command2OutputDir)
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

func runDiff(diffCommand, dir1, dir2 string) {
	cmd := exec.Command("git", diffCommand, "--no-index", dir1, dir2)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	log.Printf("Running command %s\n", strings.Join(cmd.Args, " "))
	cmd.Run()
}
