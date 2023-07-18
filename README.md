![2-men-building-a-website](website-builder.jpg)

**Warning: work in progress**

WebMaker2000 is an in-place static site generator, meaning processed files are
stored right next to their sources. This simplifies composing (source dir
layout _is_ finished website layout; static assets no longer need to be moved
around) and publishing (simply `rsync`/`git push` your whole dir). It aims to
be beginner-friendly while encouraging users to fiddle with html/css. To that
end, the core feature set is purposefully simple:

- [x] Finds all `*.dj` files, generates `*.html` in the same place
    + Per-page metadata allows using custom template
- [x] Generates home page, which is just a predefined `index.dj` + custom
  template. This means the user is free to swap in their own custom home page.
- [x] Generates RSS/Atom feed

Quality-of-life features are not neglected:

- [x] Livereload with no browser plugin (works but currently polls which is
  noisy, should probably upgrade to websockets)
- [ ] Shows user error messages on the livereloaded web page
- [ ] Just enough GUI so user doesn't have to touch a terminal
- [ ] 1-click deploy to popular static hosting targets (git push, rsync, etc.)

There's a sample site up at <https://nhanb.github.io/webmaker2000/about/> which
also further explains why this project exists.

The markup language of choice is [Djot](https://djot.net/) because it's the
only Markdown derivative that actually tries to be both unambiguous _and_
useful.

Currently works on Linux. The plan is to package for Windows & macOS too.

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
