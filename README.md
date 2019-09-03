# hugo-diff

Like `git diff`, but for built hugo sites.

## Build instructions

This project uses gomodules, so to build, do:

```sh
go build ./cmd/diff
go build ./cmd/difftool
```

To run,

```
cd your-hugo-directory
hugo-diff <ref> [<ref>]
```

## Next steps before shipping / productionizing
- Tests?
- Figure out if this works with submodules. The standard hugo install instructions are 'use submodules' so it would be good to get this right.

## Other things that would be nice for the future (roughly ordered)
- Support optional `--command` argument (so you can use it with different static site generators)
- Cleanup temp files, unless debug option specified or something
- Maybe set things up so that running without a second arg just takes the current working directory as-is, so you don't need to commit before diffing.
