package main

import (
	"encoding/xml"
	"strings"

	"golang.org/x/tools/blog/atom"
)

// TODO: Use Article's updated date instead of PostedAt.
// I need to implement Article.Meta.UpdatedAt first though.
func generateFeed(site SiteMetadata, posts []Article, path string) []byte {
	siteAddr := site.Address
	if !strings.HasSuffix(siteAddr, "/") {
		siteAddr += "/"
	}
	var entries []*atom.Entry
	for _, p := range posts {
		entries = append(entries, &atom.Entry{
			ID:        siteAddr + p.WebPath,
			Link:      []atom.Link{{Href: siteAddr + p.WebPath}},
			Title:     p.Meta.Title,
			Published: atom.Time(p.Meta.PostedAt),
			Updated:   atom.Time(p.Meta.PostedAt),
		})
	}

	feed := atom.Feed{
		ID:      siteAddr,
		Title:   site.Name,
		Updated: atom.Time(posts[0].Meta.PostedAt),
		Entry:   entries,
		Author: &atom.Person{
			Name:  site.Author.Name,
			URI:   site.Author.URI,
			Email: site.Author.Email,
		},
		Link: []atom.Link{{Rel: "self", Href: path}},
	}

	result, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		panic(err)
	}
	return result
}
