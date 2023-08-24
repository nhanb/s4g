`s4g` (Stupidly Simple Static Site Generator) is an in-place static site
generator, meaning processed files are stored right next to their sources.
This simplifies composing (source dir layout _is_ finished website layout;
static assets no longer need to be moved around) and publishing
(simply `rsync`/`git push` your whole dir).
It aims to be beginner-friendly while encouraging users to fiddle with
html/css. To that end, the core feature set is intentionally simple:

- [x] Finds all `*.dj` files, generates `*.html` in the same place
    + Per-page metadata allows using custom template
- [x] Generates home page, which is just a predefined `index.dj` + custom
  template. This means the user is free to swap in their own custom home page.
- [x] Generates RSS/Atom feed
- [x] Generates redirects from a `redirects.txt` file
- [x] Post series
- [x] Arbitrary navbar links, custom footer

Quality-of-life features:

- [x] Livereload with no browser plugin (works but currently polls which is
  noisy, should probably upgrade to websockets)
- [x] Shows user error messages on the livereloaded web page

There's a sample site up at <https://nhanb.github.io/s4g/about/>.
I'm also using s4g to generate my own blog: <https://hi.imnhan.com/s4g>.

The markup language of choice is [Djot](https://djot.net/) because it's the
only Markdown derivative that actually tries to be both unambiguous _and_
extensible.

Currently works on Linux. It should also work on macOS and Windows (probably
with a tweak to how it calls nodejs), but I'm not testing that.

Requirements: `go` (build), `node` (runtime).

```sh
sudo pacman -Syu go nodejs
go install go.imnhan.com/s4g@latest

# Create new site
s4g new -f ~/my-blog

# Run program, which:
# - listens to changes and automatically re-generates
# - starts a local HTTP server for preview, also livereloads on changes
cd ~/my-blog
s4g
```

# Documentation

Distant TODO. Maybe I'll write a blog post and call it a day.

# Potential nice-to-haves

- When cleaning up outdated files from manifest, delete empty dirs too
- Checked internal links (link to other article, to other article's asset)
- Warn when linking to redirected content
- Minify/prettify HTML

# Dev notes

`writeablefs` is admittedly an unnecessary abstraction. The prospect of
replacing the native filesystem backend with something else is highly unlikely
in the first place. I simply wanted to get used to the ergonomics of
programming portably against an FS interface. I've already violated that in my
recent commits anyway. When I'm sufficiently bored I'll most likely remove it
wholesale.
