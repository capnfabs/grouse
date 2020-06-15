package pkg

import (
	"testing"

	"github.com/capnfabs/grouse/internal/exec"
	"github.com/capnfabs/grouse/internal/git"
	"github.com/capnfabs/grouse/internal/out"
	"github.com/capnfabs/grouse/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Here's two examples.
var WrittenCommitRefs []git.Hash = []git.Hash{
	"f2999e8ac89b88a590b9902e9283dc76790ba384",
	"04beca3bd964b7049f34b037d3c86c8edd991b36",
}

func mockWriteRepo() *mocks.WriteableRepository {
	r := new(mocks.WriteableRepository)
	r.On("RootDir").Return("/tmp/repo")
	// Gives a different value from WrittenCommitRefs each time.
	counter := 0
	r.On("CommitEverythingInWorktree", mock.Anything).Return(func(message string) git.Hash {
		val := WrittenCommitRefs[counter]
		counter++
		return val
	}, nil)
	r.On("ClearSourceControlledFilesFromWorktree").Return(nil)
	return r
}

func resolve(r *mocks.Repository, hash string, userRef string) *mocks.ResolvedUserRef {
	commit := new(mocks.ResolvedCommit)
	commit.On("Repo").Return(r)
	commit.On("Hash").Return(git.Hash(hash))

	ref := new(mocks.ResolvedUserRef)
	ref.On("Commit").Return(commit)
	ref.On("UserRef", userRef)
	return ref
}

func mockReadRepo() *mocks.Repository {
	r := new(mocks.Repository)
	wt := new(mocks.WorktreeRepository)
	wt.On("RootDir").Return("/tmp/worktree")
	wt.On("Remove").Return(nil)
	wt.On("Checkout", mock.Anything).Return(nil)

	ref := resolve(r, "123123123123123123123", "tags/nope")

	r.On("RootDir").Return("/tmp/repo")
	r.On("ResolveCommit", mock.Anything).Return(ref, nil)
	r.On("RecursiveSharedCloneTo", mock.Anything).Return(wt, nil)
	return r
}

type m struct {
	Exec *mock.Mock
	Run  *mock.Mock
}

func installFixtures() (m, func()) {
	out.Reinit(true)

	exec, cexec := installMockExec()
	run, crun := installMockRun()

	return m{Exec: exec, Run: run}, func() {
		out.Reinit(false)
		cexec()
		crun()
	}
}

func installMockExec() (*mock.Mock, func()) {
	mockExec := mock.Mock{}
	old := exec.Exec
	exec.Exec = func(workDir string, args ...string) exec.CmdResult {
		res := mockExec.Called(workDir, args)
		return res.Get(0).(exec.CmdResult)
	}

	mockExec.On("func1", mock.Anything, mock.Anything).Return(exec.CmdResult{
		StdErr: "",
		StdOut: "",
		Err:    nil,
	})
	return &mockExec, func() {
		exec.Exec = old
	}
}

func installMockRun() (*mock.Mock, func()) {
	mockRun := mock.Mock{}
	old := exec.Run
	exec.Run = func(cmd *exec.Cmd) error {
		res := mockRun.Called(cmd)
		return res.Error(0)
	}

	mockRun.On("func1", mock.Anything).Return(nil)

	return &mockRun, func() {
		exec.Run = old
	}
}

func TestPassthroughBuildArgs(t *testing.T) {
	runnerMocks, cleanup := installFixtures()
	defer cleanup()

	mockGit := new(mocks.Git)
	mockGit.On("NewRepository", mock.Anything).Return(mockWriteRepo(), nil)
	mockGit.On("OpenRepository", mock.Anything).Return(mockReadRepo(), nil)
	mockGit.On("GetRelativeLocation", mock.Anything).Return("potato/tomato", nil)

	args := cmdArgs{
		repoDir:      "",
		diffCommand:  "diff",
		commits:      []string{"HEAD^", "HEAD"},
		diffArgs:     []string{},
		gitArgs:      []string{},
		buildArgs:    []string{"--here-is-a-build-arg", "message text with 'apostrophes'"},
		debug:        false,
		keepWorktree: false,
	}
	runMain(mockGit, args)

	cmds := findCmdsMatchingArgs(runnerMocks.Run.Calls, "hugo")
	for _, cmd := range cmds {
		assert.Equal(
			t,
			[]string{"hugo", "--here-is-a-build-arg", "message text with 'apostrophes'", "--destination=/tmp/repo"},
			cmd.Args)
	}
	// Two build commands
	assert.Equal(t, 2, len(cmds))
}

func matchHash(hash string) interface{} {
	return mock.MatchedBy(func(c git.ResolvedCommit) bool {
		return c.Hash() == git.Hash(hash)
	})
}

func TestChecksOutCorrectSrcShas(t *testing.T) {
	_, cleanup := installFixtures()
	defer cleanup()

	mockGit := new(mocks.Git)
	mockReadRepo := new(mocks.Repository)
	mockReadRepo.On("RootDir").Return("/tmp/repo")
	mockReadRepo.On("ResolveCommit", "origin/YOLO").Return(resolve(mockReadRepo, "111de18a818abd90ebdf1e5628820cd10d4e3efe", "origin/YOLO"), nil)
	mockReadRepo.On("ResolveCommit", "HEAD").Return(resolve(mockReadRepo, "301e857edf2f032ff58cd812fca526c5bae64569", "HEAD"), nil)

	wt := new(mocks.WorktreeRepository)
	wt.On("RootDir").Return("/tmp/worktree")
	wt.On("Remove").Return(nil)
	wt.On("Checkout", mock.Anything).Return(nil)
	mockReadRepo.On("RecursiveSharedCloneTo", mock.Anything).Return(wt, nil)

	mockGit.On("NewRepository", mock.Anything).Return(mockWriteRepo(), nil)
	mockGit.On("OpenRepository", mock.Anything).Return(mockReadRepo, nil)
	mockGit.On("GetRelativeLocation", mock.Anything).Return("potato/tomato", nil)

	args := cmdArgs{
		repoDir:      "",
		diffCommand:  "diff",
		commits:      []string{"origin/YOLO", "HEAD"},
		diffArgs:     []string{},
		gitArgs:      []string{},
		buildArgs:    []string{""},
		debug:        false,
		keepWorktree: false,
	}
	runMain(mockGit, args)

	wt.AssertCalled(t, "Checkout", matchHash("111de18a818abd90ebdf1e5628820cd10d4e3efe"))
	wt.AssertCalled(t, "Checkout", matchHash("301e857edf2f032ff58cd812fca526c5bae64569"))
	wt.AssertNumberOfCalls(t, "Checkout", 2)
}

func TestDiffArgs(t *testing.T) {
	for _, cmd := range []string{"diff", "difftool"} {
		t.Run("command_"+cmd, func(t *testing.T) {
			runnerMocks, cleanup := installFixtures()
			defer cleanup()

			mockGit := new(mocks.Git)
			mockGit.On("OpenRepository", mock.Anything).Return(mockReadRepo(), nil)
			mockGit.On("GetRelativeLocation", mock.Anything).Return("potato/tomato", nil)
			mockGit.On("NewRepository", mock.Anything).Return(mockWriteRepo(), nil)

			args := cmdArgs{
				repoDir:      "",
				diffCommand:  "diff",
				commits:      []string{"HEAD^", "HEAD"},
				gitArgs:      []string{},
				diffArgs:     []string{"hello", "--from-the-other-siiiiiiiiiiide"},
				buildArgs:    []string{""},
				debug:        false,
				keepWorktree: false,
			}
			runMain(mockGit, args)
			diffCmds := findCmdsMatchingArgs(runnerMocks.Run.Calls, "git", "diff")
			assert.Equal(t, 1, len(diffCmds))
			assert.Equal(t, []string{"git", "diff", "hello", "--from-the-other-siiiiiiiiiiide", string(WrittenCommitRefs[0]), string(WrittenCommitRefs[1])}, diffCmds[0].Args)
		})
	}
}

func TestGitArgs(t *testing.T) {

	t.Run("command_", func(t *testing.T) {
		runnerMocks, cleanup := installFixtures()
		defer cleanup()

		mockGit := new(mocks.Git)
		mockGit.On("OpenRepository", mock.Anything).Return(mockReadRepo(), nil)
		mockGit.On("GetRelativeLocation", mock.Anything).Return("potato/tomato", nil)
		mockGit.On("NewRepository", mock.Anything).Return(mockWriteRepo(), nil)

		args := cmdArgs{
			repoDir:      "",
			diffCommand:  "diff",
			commits:      []string{"HEAD^", "HEAD"},
			gitArgs:      []string{"hello", "--from-the-other-siiiiiiiiiiide"},
			buildArgs:    []string{""},
			debug:        false,
			keepWorktree: false,
		}
		runMain(mockGit, args)
		gitCmds := findCmdsMatchingArgs(runnerMocks.Run.Calls, "git")
		assert.Equal(t, 1, len(gitCmds))
		assert.Equal(t, []string{"git", "hello", "--from-the-other-siiiiiiiiiiide", "diff", string(WrittenCommitRefs[0]), string(WrittenCommitRefs[1])}, gitCmds[0].Args)
	})

}

func findCmdsMatchingArgs(calls []mock.Call, args ...string) []*exec.Cmd {
	matches := []*exec.Cmd{}
	for _, call := range calls {
		cmd := call.Arguments[0].(*exec.Cmd)
		if len(cmd.Args) >= len(args) && equal(cmd.Args[:len(args)], args) {
			matches = append(matches, cmd)
		}
	}
	return matches
}

func equal(sliceA, sliceB []string) bool {
	if len(sliceA) != len(sliceB) {
		return false
	}
	for i := range sliceA {
		if sliceA[i] != sliceB[i] {
			return false
		}
	}
	return true
}
