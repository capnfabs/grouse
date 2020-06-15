package git

import (
	"fmt"

	au "github.com/logrusorgru/aurora"
)

// Hash is a git SHA-1, base16 encoded
type Hash string

// NilHash is the zero value, i.e. the absence of a Hash.
const NilHash = Hash("")

// ResolvedCommit represents a commit which definitely exists within its
// associated repository.
type ResolvedCommit interface {
	Repo() Repository
	Hash() Hash
}

func (r *resolvedCommit) Repo() Repository {
	return r.repo
}
func (r *resolvedCommit) Hash() Hash {
	return r.hash
}

type resolvedCommit struct {
	repo *repository
	hash Hash
}

func (r *resolvedCommit) String() string {
	return string(r.hash[:7])
}

// ResolvedUserRef represents a user-provided commit-ish, which has then been
// resolved to a commit in the git repo. This is useful info together for
// referring back to input that the user has supplied; notably, the String()
// method returns a representation of the user's input and the SHA together.
type ResolvedUserRef interface {
	Commit() ResolvedCommit
	UserRef() string
}

type resolvedUserRef struct {
	commit  resolvedCommit
	userRef string
}

func (r *resolvedUserRef) Commit() ResolvedCommit {
	return &r.commit
}
func (r *resolvedUserRef) UserRef() string {
	return r.userRef
}

func (r *resolvedUserRef) String() string {
	return fmt.Sprintf("%s (%s)", au.Blue(r.userRef), au.Yellow(r.commit.hash[:7]))
}
