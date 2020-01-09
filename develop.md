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

Mocks:

```sh
export GOBIN=$PWD/bin
export PATH=$GOBIN:$PATH
bin/mockery -dir internal/git -all
```

See also https://github.com/vektra/mockery/issues/210

See this for installing tools: https://stackoverflow.com/a/57317864/996592

## Releasing

- Test build artifacts with: `goreleaser --snapshot --skip-publish --rm-dist`
- Tag with git: `git tag v[whatever]`
- Push tags: `git push --tags`
- Actually do the release: `goreleaser --rm-dist`

## TODOs
- Figure out how to make this play nice with snapcraft Hugo.
  - You can run /snap/hugo/current/bin/hugo and it will work; but that's kinda violating a bunch of safety expectations and I don't love that. It probably also won't work for binaries that aren't statically compiled; or if Hugo needs to e.g. shell out for anything that's not also installed at a system level.
  - Alternatively, could do temporary storage in the HOME directory? We'd just have to remember to clean it up later. This is what snapcraft does (see e.g. https://github.com/snapcore/snapcraft/pull/1519/files).
    - Could do this either automatically or manually based on whether we detect snapcraft is in use; detecting that could be complicated though?
  - A complicating factor for this is that some GUI diff tools early-return; and if that's the case it could pretty plausibly result in the files getting deleted before they are actually used. (e.g. if the user specifies `grouse --tool --diffargs=--dir`, we'd also need to specify `--no-symlink` so that there's still something to compare in the end).
  - And a solution to that could be something _horrific_ like watching for processes spawned by the git diff invocation, or something _less_ horrific, like just deleting diffs more than a day old or something. All the intermediate files can be happily cleaned up :)
  - Can we detect use of snapcraft to determine where we want to store files? _Probably_. `which hugo` --> `/snap/bin/hugo`, ls -l  `which hugo` prints `/usr/bin/snap`. So we can probably do something like, check that the binary name is 'snap' as a heuristic for this.
  - Can other apps bundled into a snap break out of the sandbox? e.g. why can `git` included in the Hugo snap access the .git directories... oh, it's because you can't access dotfiles in the root home dir but you can in other dirs. ok. Note that you _also_ can't access the / directory. ok then.
  - Hugo seems to be the only static site generator that's available via snapcraft at the moment; Jekyll / Gatsby / Pelican for example are all unavailable via snaps (there's a draft floating around for Jekyll but it's two years old and never got published).
- Windows test / support

## Things that would be nice for the future (roughly ordered)
- Tests that commands get run correctly.
- Support optional `--command` argument (so you can use it with different static site generators, not just hugo)
- Try to autodetect a few common static site generators (hugo, jekyll, gatsby)
- Cache historical builds in the temp dir.
- Cleanup checkout-ed project files, unless debug option specified or something
- Maybe set things up so that running without a second arg just takes the current working directory as-is, so you don't need to commit before diffing.
- Maybe? Force the same timestamp so that themes which use timestamps won't generate false-positives everywhere.
