package checkout

import (
	"fmt"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

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

type ResolvedCommit interface {
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

func ResolveUserRef(repo *git.Repository, userRef string) (ResolvedCommit, error) {
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

// Internal version so the types work properly
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

func ResolveHash(repo *git.Repository, hash plumbing.Hash) (ResolvedCommit, error) {
	return resolveHash(repo, hash)
}

func (r *resolvedUserRef) String() string {
	return fmt.Sprintf("%s (%s)", r.userRef, r.hash.String()[:7])
}
