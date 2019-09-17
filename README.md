# Grouse

Like `git diff`, but for generated [Hugo](https://gohugo.io) sites.

Imagine that, every time you pushed changes to your Hugo site, you also version-controlled the generated HTML/CSS/JS files. Then, when you were changing anything important on your site, you could also run `git diff` to see whether your changes had unintended side effects.

Grouse approximates that process by checking out previous commits, running Hugo, and then running `git diff` against the result.

## Install

If you're on Mac OSX and have [homebrew](https://brew.sh) installed:

```sh
brew install capnfabs/tap/grouse
```

Otherwise, you can download the [latest release for your platform directly from the releases page](https://github.com/capnfabs/grouse/releases/latest).

## Usage

### Quick reference

```sh
cd your-hugo-site
git log  # Should be a git repo.
# Show the difference between the generated output on these two commit references.
grouse commitRefA commitRefB
```

### Specifying commits

Anything you can `git diff` against works as a commit reference for Grouse:
- hashes (e.g. `8c90155d4`)
- branch names (e.g. `feature/photo-albums`)
- parent commits (e.g. `HEAD^` - the previous commit)
- tags (`v0.1`)
- probably other things too!

### Command-line flags

- `grouse --tool` runs `git difftool` instead of `git diff`
- Pass additional args to the Hugo builds with `--buildargs`
- Pass additional args to the `git diff` command with `--diffargs`

## Development instructions

Instructions for developers are in [develop.md](develop.md).
