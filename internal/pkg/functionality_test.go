package pkg

import (
	"testing"

	"github.com/capnfabs/grouse/internal/exec"
	"github.com/capnfabs/grouse/internal/git"
	"github.com/capnfabs/grouse/mocks"
	"github.com/stretchr/testify/mock"
)

/* test ideas:
- A bunch of end-2-end tests:
  - Normal repo
  - Really big repo with lots of files
  - A directory without a git repo
  - A repo with submodules in use in the tree, without submodules available
  - A repo that uses the git extensions to Hugo

- Unit tests:
	- Test that args are passed thru to hugo
	- Test that args are passed thru to git diff
	- Test --tool
	- Test console output.
*/

type MockGit struct {
	mock.Mock
}

func (m *MockGit) NewRepository(dst string) (git.Repository, error) {
	args := m.Called(dst)
	return args.Get(0).(git.Repository), args.Error(1)
}

func (m *MockGit) OpenRepository(repoDir string) (git.Repository, error) {
	args := m.Called(repoDir)
	return args.Get(0).(git.Repository), args.Error(1)
}

func (m *MockGit) GetRelativeLocation(currentDir string) (string, error) {
	args := m.Called(currentDir)
	return args.String(0), args.Error(1)
}

func mockWriteRepo() *mocks.Repository {
	r := new(mocks.Repository)
	r.On("RootDir").Return("/tmp/repo")
	// TODO: make this hash value change?
	r.On("CommitEverythingInWorktree", mock.Anything).Return("123123d", nil)
	r.On("ClearSourceControlledFilesFromWorktree").Return(nil)
	return r
}

func mockReadRepo() *mocks.Repository {
	r := new(mocks.Repository)
	wt := new(mocks.Worktree)
	wt.On("Location").Return("/tmp/worktree")
	wt.On("Remove").Return(nil)
	wt.On("Checkout", mock.Anything).Return(nil)

	commit := new(mocks.ResolvedCommit)
	commit.On("Repo").Return(r)
	commit.On("Hash").Return("123123123123123123123")

	ref := new(mocks.ResolvedUserRef)
	ref.On("Commit").Return(commit)
	ref.On("UserRef", "tags/nope")

	r.On("RootDir").Return("/tmp/repo")
	r.On("ResolveCommit", mock.Anything).Return(ref, nil)
	r.On("AddWorktree", mock.Anything).Return(wt, nil)
	return r
}

func TestPassthroughBuildArgs(t *testing.T) {

	mockExec := mock.Mock{}
	exec.Exec = func(workDir string, args ...string) exec.CmdResult {
		res := mockExec.Called(workDir, args)
		return res.Get(0).(exec.CmdResult)
	}

	mockExec.On("func1", mock.Anything, mock.Anything).Return(exec.CmdResult{
		StdErr: "",
		StdOut: "",
		Err:    nil,
	})

	mockGit := new(MockGit)
	mockGit.On("OpenRepository", mock.Anything).Return(mockReadRepo(), nil)
	mockGit.On("GetRelativeLocation", mock.Anything).Return("potato/tomato", nil)
	mockGit.On("NewRepository", mock.Anything).Return(mockWriteRepo(), nil)

	args := cmdArgs{
		repoDir:      "",
		diffCommand:  "diff",
		commits:      []string{"HEAD^", "HEAD"},
		diffArgs:     []string{},
		buildArgs:    []string{"--here-is-a-build-arg", "message text with 'apostrophes'"},
		debug:        false,
		keepWorktree: false,
	}
	runMain(mockGit, args)
}
