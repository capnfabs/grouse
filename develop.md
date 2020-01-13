# Developing Grouse

## Build instructions

**FYI**: This project uses gomodules, so clone it to a project that's not in your `$GOPATH`.

To build, run:

```sh
go build -o grouse
```

To run,

```
cd your-hugo-directory
grouse <ref> [<ref>]
```

## Tests

There are two kinds of tests:

- fast-ish unit tests
- some really fat integration tests that require `git-lfs`, `hugo`, and `git` to be installed. These unzip entire repos stored in zip files and then run grouse on them, and check that the output hasn't changed from the snapshot saved in the repo (then you can git diff the output files to see what's changed).

```sh
# Unit tests!
go test -short ./...

# Also include tests on 180MB repos which may clone random stuff from github :-/
go test ./...
```

### Running Integration Tests on CircleCI

- The integration tests have some ugly dependencies, so I put them all in a docker container.
- To push a new version of the docker container:

```sh
# Bump as required
VERSION=0.1
docker build -t "capnfabs/grouse-integration:$VERSION" .circleci/images/primary
docker push
# Now update .circleci/config.yml to refer to the new image.
```

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

## Releasing

- Test build artifacts with: `goreleaser --snapshot --skip-publish --rm-dist`
- Tag with git: `git tag v[whatever]`
- Push tags: `git push --tags`
- Actually do the release: `goreleaser --rm-dist`


## Things that would be nice for the future (roughly ordered, see also [#enhancements](https://github.com/capnfabs/grouse/issues?q=is%3Aissue+is%3Aopen+label%3Aenhancement))
- Try to autodetect a few common static site generators (hugo, jekyll, gatsby)
- Cache historical builds in the temp dir.
- Maybe? Force the same timestamp so that themes which use timestamps won't generate false-positives everywhere.
