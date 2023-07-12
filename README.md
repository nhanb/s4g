**Warning: work in progress**

WebMaker2000 is an in-place static site generator, meaning processed files are
stored right next to their sources. This simplifies composing (source dir
layout _is_ finished website layout) and publishing (simply `rsync`/`git push`
your whole dir).

There's a sample site up at <https://nhanb.github.io/webmaker2000/about/> which
also further explains why this project exists.

The markup language of choice is [Djot](https://djot.net/) because it's the
only Markdown derivative that actually tries to be both unambiguous _and_
useful.

Requirements: `go` (build), `node` (runtime).

```sh
# Install
go install go.imnhan.com/webmaker2000@latest

# Create new site
webmaker2000 new -f ~/my-blog

# Run program, which:
# - listens to changes and automatically re-generates
# - starts a local HTTP server for preview, also livereloads on changes
webmaker2000 serve -f ~/my-blog
```

GUI Coming Soon (tm).
