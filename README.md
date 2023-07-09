**Warning: work in progress**

WebMaker2000 is an in-place static site generator, meaning processed files are
stored right next to their sources. This simplifies composing (source dir
layout _is_ finished website layout) and publishing (simply `rsync`/`git push`
your whole dir).

The markup language of choice is [Djot](https://djot.net/) because it's the
only Markdown derivative that actually tries to be both unambiguous _and_
useful.

Requirements: `go`, `node`.

```sh
# Install
go install go.imnhan.com/webmaker2000@latest

# Create new site
webmaker2000 -new ~/my-blog

# Run program
webmaker2000 -folder ~/my-blog
```
