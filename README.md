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

## Next steps:
- Tests?
- Figure out if this works with submodules. The standard hugo install instructions are 'use submodules' so it would be good to get this right.
- Pass arguments through to diff
- Maybe set things up so that running without a second arg just takes the current working directory as-is, so you don't need to commit before diffing.
- Support optional `--command` argument (so you can use it with different static site generators)
-  Remove source copies, unless debug option specified or something
