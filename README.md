go-binary-deps
====

Go binary deps captures the transitive dependencies of go binaries in a folder.  Why would this be useful?  You can use this
to determine _changesets_ from a git commit. Imagine you have a large monorepo and you want to find out which sets of files affects
which binaries (maybe to deploy just those binaries). You can use this tool to find those binaries local transitive references
and correlate them across the git changes.

Example usage:

```
binaries := Binaries("..", Resolution{
		LocalPrefix:  "github.com/paradoxical-io",
		IncludeTests: true,
	})
```
Which when iterated prints:

```
cmd1
  github.com/paradoxical-io/go-binary-deps/util
  github.com/paradoxical-io/go-binary-deps/util2
cmd2
cmd3
```
