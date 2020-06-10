package pkg

import (
	"fmt"
	"os"

	"github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
)

// Internal interface used for testing
type flagSet interface {
	GetBool(string) (bool, error)
	GetString(string) (string, error)
	Args() []string
}

func parseArgs(flags flagSet) (*cmdArgs, error) {
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

	debug, err := flags.GetBool("debug")
	check(err)

	diffArgsStr, err := flags.GetString("diffargs")
	check(err)

	keepWorktree, err := flags.GetBool("keep-cache")
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

	return &cmdArgs{
		repoDir:      repoDir,
		diffCommand:  diffCommand,
		commits:      commits,
		diffArgs:     diffArgs,
		buildArgs:    buildArgs,
		debug:        debug,
		keepWorktree: keepWorktree,
	}, nil
}

type cmdArgs struct {
	repoDir     string
	diffCommand string
	commits     []string
	diffArgs    []string
	buildArgs   []string
	debug       bool
	// TODO: rename
	keepWorktree bool
}
