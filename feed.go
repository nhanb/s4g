package main

import (
	"encoding/xml"
	"strings"

	"golang.org/x/tools/blog/atom"
)

// TODO: Use Article's updated date instead of PostedAt.
// I need to implement Article.UpdatedAt first though.
func generateFeed(site SiteMetadata, posts []Article, path string) []byte {
	siteAddr := site.Address
	if !strings.HasSuffix(siteAddr, "/") {
		siteAddr += "/"
	}
	var entries []*atom.Entry
	for _, p := range posts {
		// trim WebPath's leading slash because siteAddr already has one
		link := siteAddr + p.WebPath[1:]
		entries = append(entries, &atom.Entry{
			ID:        link,
			Link:      []atom.Link{{Href: link}},
			Title:     p.Title,
			Published: atom.Time(p.PostedAt),
			Updated:   atom.Time(p.PostedAt),
		})
	}

	feed := atom.Feed{
		ID:      siteAddr,
		Title:   site.Name,
		Updated: atom.Time(posts[0].PostedAt),
		Entry:   entries,
		Author: &atom.Person{
			Name:  site.AuthorName,
			URI:   site.AuthorURI,
			Email: site.AuthorEmail,
		},
		Link: []atom.Link{{Rel: "self", Href: path}},
	}

	result, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		panic(err)
	}
	return result
}
