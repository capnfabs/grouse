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

```sh
go test ./...
```

- They're _not_ very comprehensive at the moment. I found testing in Go really hard.
- To compress a hugo site repo for the tests, rename the directory to `input` and then run e.g. `zip -r tiny.zip input`

## Releasing

- Test build artifacts with: `goreleaser --snapshot --skip-publish --rm-dist`
- Tag with git: `git tag v[whatever]`
- Push tags: `git push --tags`
- Actually do the release: `goreleaser --rm-dist`

## Things that would be nice for the future (roughly ordered)
- Tests that commands get run correctly.
- Support optional `--command` argument (so you can use it with different static site generators, not just hugo)
- Try to autodetect a few common static site generators (hugo, jekyll, gatsby)
- Cache historical builds in the temp dir.
- Cleanup checkout-ed project files, unless debug option specified or something
- Maybe set things up so that running without a second arg just takes the current working directory as-is, so you don't need to commit before diffing.
- Maybe? Force the same timestamp so that themes which use timestamps won't generate false-positives everywhere.
