# Grouse

[![Grouse Build](https://circleci.com/gh/capnfabs/grouse.svg?style=shield)](https://circleci.com/gh/capnfabs/grouse)

Like `git diff`, but for generated [Hugo](https://gohugo.io) sites.

Imagine that, every time you pushed changes to your Hugo site, you also version-controlled the generated HTML/CSS/JS files. Then, when you were changing anything important on your site, you could also run `git diff` to see whether your changes had unintended side effects.

Grouse approximates that process by checking out previous commits, running Hugo on them, and then running `git diff` on the outputs.

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

### Usage tips

- `grouse --diffargs="--stat"` will give you a short list of which files have changed and how much they've changed by:

    ```
    $ grouse --diffargs="--stat" master

    index.html                       |  4 ++++
    index.xml                        | 11 +++++++++-
    posts/index.xml                  | 11 +++++++++-
    posts/tawny-shoulders/index.html | 47 ++++++++++++++++++++++++++++++++++++++++
    sitemap.xml                      |  9 ++++++--
    5 files changed, 78 insertions(+), 4 deletions(-)
    ```

    This is a useful sanity-check before pushing new commits to production.

- Setting up a good diff tool will make the output of `git diff` much easier to work with. I recommend:
  - [Kaleidoscope](https://www.kaleidoscopeapp.com/), which is paid, and OSX only, but really easy-to-use
  - [Meld](http://meldmerge.org/), which is free and cross-platform. It's especially good when used with `--dir`, i.e. `grouse HEAD^ --tool --diffargs='--tool=meld --dir'`.

## Development instructions

Instructions for developers are in [develop.md](develop.md).

## Not working / Questions / Comments?

Thanks for your feedback! [File an issue](issues) or [contact me](https://capnfabs.net/contact).
