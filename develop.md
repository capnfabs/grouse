# Developing Grouse

## Build instructions

**FYI**: This project uses gomodules, so clone it to a project that's not in your `$GOPATH`.

To build, run:

```sh
go build -o grouse
```

To run,

```sh
cd your-hugo-directory
grouse <ref> [<ref>]
```

My standard build and run iteration loop command is:

```sh
go build -o grouse && (G=`pwd`/grouse; cd ../capnfabs.github.io; $G --debug --keep-cache 'HEAD^^^' 'HEAD' )
```

## Tests

There are two kinds of tests:

- fast-ish unit tests
- some fat integration tests that require `hugo`, and `git` to be installed, and an internet connection. These unzip entire repos stored in zip files and then run grouse on them, and check that the output hasn't changed from the snapshot saved in the repo (then you can git diff the output files to see what's changed).

```sh
# Unit tests!
go test -short ./...

# Also include integration tests, which may clone random stuff from github and take ~20 seconds
go test ./...
```

These all get run automatically on CircleCI every commit (I think).

### Mocks in Unit Tests

These are done with `mockery` and `testify`. Here's how (sorry this is badly documented, future me):

```sh
export GOBIN=$PWD/bin
# Run something to install the dependencies? I've forgotten what exactly.
# Now every time you want to generate mocks.
export PATH=$GOBIN:$PATH
bin/mockery -dir internal/git -all
```

- See also https://github.com/vektra/mockery/issues/210
- See this for installing tools: https://stackoverflow.com/a/57317864/996592


### Testing on Debian Stable

I develop grouse on a Mac, but I want to support Linux users, and it's not super hard to test. I test releases on Debian Stable because they're probably the oldest possible version of `git` still in use.

Here's how:

```sh
docker run -it --mount type=bind,source=$(pwd),target=/app,readonly golang:1.14-buster
apt update && apt install unzip git curl
export HUGO_RELEASE=https://github.com/gohugoio/hugo/releases/download/v0.72.0/hugo_0.72.0_Linux-64bit.deb
curl -L $HUGO_RELEASE -o hugo.deb && dpkg -i hugo.deb && rm hugo.deb
go test ./...
```

This is basically (maybe completely?) identical to what runs on CircleCI.

## Releasing

- Build artefacts for test with: `goreleaser --snapshot --skip-publish --rm-dist`
- Tag with git: `git tag v[whatever]`
- Push tags: `git push --tags`
- Actually do the release: `goreleaser --rm-dist`

## Things that would be nice for the future (roughly ordered, see also [#enhancements](https://github.com/capnfabs/grouse/issues?q=is%3Aissue+is%3Aopen+label%3Aenhancement))
- Try to autodetect a few common static site generators (hugo, jekyll, gatsby)
- Cache historical builds + repos in the temp dir.
