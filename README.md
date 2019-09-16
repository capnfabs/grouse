# Grouse

Like `git diff`, but for the output of Hugo sites.

## Build instructions

FYI: This project uses gomodules, so clone it to a project that's not in your `$GOPATH`.

To build, do:

```sh
./scripts/build.sh
```

To run,

```
cd your-hugo-directory
grouse <ref> [<ref>]
```

## Tests

Tests!? Yay! Tests!

```
go test ./...
```

## Next steps before shipping / productionizing
- Tests? For a start, let's just do integration tests by unzipping zip files and then ensuring that the output makes sense.
  - Can't do this because it depends upon running Hugo, and I don't want to run Hugo. I _can_ check that everything in the leadup makes sense, and that everything _afterwards_ makes sense, though.
- Check through all command output, switch printlns to logs

## Other things that would be nice for the future (roughly ordered)
- Support optional `--command` argument (so you can use it with different static site generators, not just hugo)
- Cleanup checkout-ed project files, unless debug option specified or something
- Try to autodetect a few common static site generators (hugo, jekyll, gatsby)
- Cache historical builds in the temp dir.
- Maybe set things up so that running without a second arg just takes the current working directory as-is, so you don't need to commit before diffing.
- Maybe? Force the same timestamp so that themes which use timestamps won't generate false-positives everywhere.
