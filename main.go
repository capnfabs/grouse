package main

import (
	"fmt"
	"os"

	"github.com/capnfabs/grouse/internal/out"
	"github.com/capnfabs/grouse/internal/pkg"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionString() string {
	return fmt.Sprintf("%v, commit %v, built at %v", version, commit, date)
}

func main() {
	rootCmd.Flags().String("gitargs", "", "Arguments to pass on to 'git'")
	rootCmd.Flags().String("diffargs", "", "Arguments to pass on to 'git diff'")
	rootCmd.Flags().String("buildargs", "", "Arguments to pass on to the hugo build command")
	rootCmd.Flags().BoolP("tool", "t", false, "Invoke 'git difftool' instead of 'git diff'")
	rootCmd.Flags().Bool("debug", false, "Enables additional logging")
	rootCmd.Flags().Bool("keep-worktree", false, "Keeps the source worktree around after running grouse. Useful for debugging and development, but adds cruft to your git repo")
	rootCmd.Flags().MarkHidden("keep-worktree")
	if err := rootCmd.Execute(); err != nil {
		out.Outln(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "grouse [flags] <commit> [<other-commit>]",
	Version: versionString(),
	Short:   "Diffs the output of a given Hugo git repo at different commits.",
	Long: `Diffs the output of a given Hugo git repo at different commits.

Imagine that on every commit of your Hugo site, you'd generated the site and
stored that in version control. Then, you could see exactly what's changed in
your generated site between different commits.

Grouse approximates that process.`,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		pkg.RunRootCommand(cmd)
	},
}
