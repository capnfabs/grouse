package pkg

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
)

type flags map[string]interface{}

func (f flags) GetBool(key string) (bool, error) {
	return f[key].(bool), nil
}

func (f flags) GetString(key string) (string, error) {
	return f[key].(string), nil
}

func (f flags) Args() []string {
	return f["_args"].([]string)
}

func defaultFlags() flags {
	return flags{
		"gitargs":       "--potato 'excellent'",
		"diffargs":      "--potato 'excellent'",
		"buildargs":     "--carrot",
		"tool":          true,
		"_args":         []string{"b1234553", "HEAD^"},
		"keep-worktree": false,
		"debug":         false,
	}
}

func TestArgParsingBaseCase(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	context, err := parseArgs(f)
	c.Check(err, qt.IsNil)
	c.Check(context.repoDir, qt.Not(qt.Equals), "")
	c.Check(context.diffCommand, qt.Equals, "difftool")
	c.Check(context.commits, qt.DeepEquals, []string{"b1234553", "HEAD^"})
	c.Check(context.diffArgs, qt.DeepEquals, []string{"--potato", "excellent"})
	c.Check(context.gitArgs, qt.DeepEquals, []string{"--potato", "excellent"})
	c.Check(context.buildArgs, qt.DeepEquals, []string{"--carrot"})
}

func TestArgParsingTool(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["tool"] = false
	context, err := parseArgs(f)
	c.Check(err, qt.IsNil)
	c.Check(context.diffCommand, qt.Equals, "diff")
}

func TestArgParsingBadEscapedDiffArgs(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["diffargs"] = "--hello='wut" // missing single quote
	context, err := parseArgs(f)
	c.Check(context, qt.IsNil)
	c.Check(err, qt.ErrorMatches, `.*--diffargs:.*`)
}

func TestArgParsingBadEscapedGitArgs(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["gitargs"] = "--hello='wut" // missing single quote
	context, err := parseArgs(f)
	c.Check(context, qt.IsNil)
	c.Check(err, qt.ErrorMatches, `.*--gitargs:.*`)
}

func TestArgParsingBadEscapedBuildArgs(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["buildargs"] = "--hello=\"wut" // missing double quote
	context, err := parseArgs(f)
	c.Check(context, qt.IsNil)
	c.Check(err, qt.ErrorMatches, `.*--buildargs:.*`)
}

func TestHandlesOneCommit(t *testing.T) {
	c := qt.New(t)
	f := defaultFlags()
	f["_args"] = []string{"b1234553"}
	context, err := parseArgs(f)
	c.Check(err, qt.IsNil)
	c.Check(context.commits, qt.DeepEquals, []string{"b1234553", "HEAD"})
}

func TestHandlesWrongNumberOfCommits(t *testing.T) {
	c := qt.New(t)
	for _, testcase := range [][]string{[]string{}, []string{"b1234553", "HEAD^^", "HEAD"}} {
		subtest := fmt.Sprintf("With%dArgs", len(testcase))
		c.Run(subtest, func(c *qt.C) {
			f := defaultFlags()
			f["_args"] = testcase
			context, err := parseArgs(f)
			c.Check(context, qt.IsNil)
			c.Check(err, qt.ErrorMatches, `Requires one or two git references.*got \d`)
		})
	}
}
