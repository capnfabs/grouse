package git

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/capnfabs/grouse/internal/exec"
)

// Git is an interface to a subset of Git functionality.
type Git interface {
	NewRepository(dst string) (WriteableRepository, error)
	OpenRepository(repoDir string) (Repository, error)
	GetRelativeLocation(currentDir string) (string, error)
}

type gitVersion struct {
	major int
	minor int
	patch int
}

func (v *gitVersion) isNewerThanOrEqualTo(major, minor, patch int) bool {
	if v.major > major {
		return true
	}
	if v.major == major {
		if v.minor > minor {
			return true
		}
		if v.minor == minor {
			if v.patch >= patch {
				return true
			}
		}
	}
	return false
}

var noVersion = gitVersion{0, 0, 0}

func parseVersionString(vs string) gitVersion {
	parts := strings.Split(vs, ".")
	if len(parts) != 3 {
		return noVersion
	}
	major, errA := strconv.Atoi(parts[0])
	minor, errB := strconv.Atoi(parts[1])
	patch, errC := strconv.Atoi(parts[2])
	if errA != nil || errB != nil || errC != nil {
		return noVersion
	}
	return gitVersion{major, minor, patch}
}

var versionRegexp = regexp.MustCompile(`^git version (\d+\.\d+\.\d+)`)

// NewGit returns a new git interface.
func NewGit() Git {
	cmd := exec.Exec("", "git", "version")
	submatches := versionRegexp.FindStringSubmatch(cmd.StdOut)
	var version gitVersion = noVersion
	if submatches != nil {
		version = parseVersionString(submatches[1])
	}
	return git{
		version: version,
	}
}

type git struct {
	version gitVersion
}

var (
	// ErrRepoExists means that the given directory already has a git repository
	// in it, and a new repository can't be created there.
	ErrRepoExists = errors.New("Repo already exists")
)

// NewRepository creates a new git repository in the given directory.
func (g git) NewRepository(dst string) (WriteableRepository, error) {
	_, err := g.OpenRepository(dst)
	if err == nil {
		return nil, ErrRepoExists
	}
	cmd := exec.Exec(dst, "git", "init")
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	return &writeableRepo{
		repository{rootDir: dst, gitInterface: &g},
	}, nil
}

// OpenRepository opens an existing git repository in repoDir. If repoDir is a
// subdirectory in the repository, OpenRepository walks up the file tree to find
// the git repo.
func (g git) OpenRepository(repoDir string) (Repository, error) {
	return g.openRepository(repoDir)
}

func (g git) openRepository(repoDir string) (*repository, error) {
	cmd := exec.Exec(repoDir, "git", "rev-parse", "--show-toplevel")
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	rootDir := cmd.StdOut
	return &repository{
		rootDir:      rootDir,
		gitInterface: &g,
	}, nil
}

// GetRelativeLocation gets the relative path from the root of a git repo to
// currentDir. e.g. if there's a git repo in ~/hello, and currentDir is
// ~/hello/potato/tomato, returns "potato/tomato".
func (g git) GetRelativeLocation(currentDir string) (string, error) {
	cmd := exec.Exec(currentDir, "git", "rev-parse", "--show-prefix")
	if cmd.Err != nil {
		return "", cmd.Err
	}
	return cmd.StdOut, nil
}
