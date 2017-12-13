// Copyright 2017 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package json

import (
	"log"
	"strings"
	"time"

	"github.com/miniflux/miniflux/helper"
	"github.com/miniflux/miniflux/model"
	"github.com/miniflux/miniflux/reader/date"
	"github.com/miniflux/miniflux/reader/sanitizer"
)

type jsonFeed struct {
	Version string     `json:"version"`
	Title   string     `json:"title"`
	SiteURL string     `json:"home_page_url"`
	FeedURL string     `json:"feed_url"`
	Author  jsonAuthor `json:"author"`
	Items   []jsonItem `json:"items"`
}

type jsonAuthor struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type jsonItem struct {
	ID            string           `json:"id"`
	URL           string           `json:"url"`
	Title         string           `json:"title"`
	Summary       string           `json:"summary"`
	Text          string           `json:"content_text"`
	HTML          string           `json:"content_html"`
	DatePublished string           `json:"date_published"`
	DateModified  string           `json:"date_modified"`
	Author        jsonAuthor       `json:"author"`
	Attachments   []jsonAttachment `json:"attachments"`
}

type jsonAttachment struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Title    string `json:"title"`
	Size     int    `json:"size_in_bytes"`
	Duration int    `json:"duration_in_seconds"`
}

func (j *jsonFeed) GetAuthor() string {
	return getAuthor(j.Author)
}

func (j *jsonFeed) Transform() *model.Feed {
	feed := new(model.Feed)
	feed.FeedURL = j.FeedURL
	feed.SiteURL = j.SiteURL
	feed.Title = strings.TrimSpace(j.Title)

	if feed.Title == "" {
		feed.Title = feed.SiteURL
	}

	for _, item := range j.Items {
		entry := item.Transform()
		if entry.Author == "" {
			entry.Author = j.GetAuthor()
		}

		feed.Entries = append(feed.Entries, entry)
	}

	return feed
}

func (j *jsonItem) GetDate() time.Time {
	for _, value := range []string{j.DatePublished, j.DateModified} {
		if value != "" {
			d, err := date.Parse(value)
			if err != nil {
				log.Println(err)
				return time.Now()
			}

			return d
		}
	}

	return time.Now()
}

func (j *jsonItem) GetAuthor() string {
	return getAuthor(j.Author)
}

func (j *jsonItem) GetHash() string {
	for _, value := range []string{j.ID, j.URL, j.Text + j.HTML + j.Summary} {
		if value != "" {
			return helper.Hash(value)
		}
	}

	return ""
}

func (j *jsonItem) GetTitle() string {
	for _, value := range []string{j.Title, j.Summary, j.Text, j.HTML} {
		if value != "" {
			return truncate(sanitizer.StripTags(value))
		}
	}

	return j.URL
}

func (j *jsonItem) GetContent() string {
	for _, value := range []string{j.HTML, j.Text, j.Summary} {
		if value != "" {
			return value
		}
	}

	return ""
}

func (j *jsonItem) GetEnclosures() model.EnclosureList {
	enclosures := make(model.EnclosureList, 0)

	for _, attachment := range j.Attachments {
		enclosures = append(enclosures, &model.Enclosure{
			URL:      attachment.URL,
			MimeType: attachment.MimeType,
			Size:     attachment.Size,
		})
	}

	return enclosures
}

func (j *jsonItem) Transform() *model.Entry {
	entry := new(model.Entry)
	entry.URL = j.URL
	entry.Date = j.GetDate()
	entry.Author = j.GetAuthor()
	entry.Hash = j.GetHash()
	entry.Content = j.GetContent()
	entry.Title = strings.TrimSpace(j.GetTitle())
	entry.Enclosures = j.GetEnclosures()
	return entry
}

func getAuthor(author jsonAuthor) string {
	if author.Name != "" {
		return strings.TrimSpace(author.Name)
	}

	return ""
}

func truncate(str string) string {
	max := 100
	str = strings.TrimSpace(str)
	if len(str) > max {
		return str[:max] + "..."
	}

	return str
}
