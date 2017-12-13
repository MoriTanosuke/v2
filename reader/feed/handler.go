// Copyright 2017 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package feed

import (
	"fmt"
	"log"
	"time"

	"github.com/miniflux/miniflux/errors"
	"github.com/miniflux/miniflux/helper"
	"github.com/miniflux/miniflux/http"
	"github.com/miniflux/miniflux/model"
	"github.com/miniflux/miniflux/reader/icon"
	"github.com/miniflux/miniflux/reader/processor"
	"github.com/miniflux/miniflux/storage"
)

var (
	errRequestFailed    = "Unable to execute request: %v"
	errServerFailure    = "Unable to fetch feed (statusCode=%d)."
	errDuplicate        = "This feed already exists (%s)."
	errNotFound         = "Feed %d not found"
	errEncoding         = "Unable to normalize encoding: %v."
	errCategoryNotFound = "Category not found for this user."
)

// Handler contains all the logic to create and refresh feeds.
type Handler struct {
	store *storage.Storage
}

// CreateFeed fetch, parse and store a new feed.
func (h *Handler) CreateFeed(userID, categoryID int64, url string, crawler bool) (*model.Feed, error) {
	defer helper.ExecutionTime(time.Now(), fmt.Sprintf("[Handler:CreateFeed] feedUrl=%s", url))

	if !h.store.CategoryExists(userID, categoryID) {
		return nil, errors.NewLocalizedError(errCategoryNotFound)
	}

	client := http.NewClient(url)
	response, err := client.Get()
	if err != nil {
		return nil, errors.NewLocalizedError(errRequestFailed, err)
	}

	if response.HasServerFailure() {
		return nil, errors.NewLocalizedError(errServerFailure, response.StatusCode)
	}

	if h.store.FeedURLExists(userID, response.EffectiveURL) {
		return nil, errors.NewLocalizedError(errDuplicate, response.EffectiveURL)
	}

	body, err := response.NormalizeBodyEncoding()
	if err != nil {
		return nil, errors.NewLocalizedError(errEncoding, err)
	}

	subscription, err := parseFeed(body)
	if err != nil {
		return nil, err
	}

	feedProcessor := processor.NewFeedProcessor(subscription)
	feedProcessor.WithCrawler(crawler)
	feedProcessor.Process()

	subscription.Category = &model.Category{ID: categoryID}
	subscription.EtagHeader = response.ETag
	subscription.LastModifiedHeader = response.LastModified
	subscription.FeedURL = response.EffectiveURL
	subscription.UserID = userID
	subscription.Crawler = crawler

	err = h.store.CreateFeed(subscription)
	if err != nil {
		return nil, err
	}

	log.Println("[Handler:CreateFeed] Feed saved with ID:", subscription.ID)

	icon, err := icon.FindIcon(subscription.SiteURL)
	if err != nil {
		log.Println(err)
	} else if icon == nil {
		log.Printf("No icon found for feedID=%d\n", subscription.ID)
	} else {
		h.store.CreateFeedIcon(subscription, icon)
	}

	return subscription, nil
}

// RefreshFeed fetch and update a feed if necessary.
func (h *Handler) RefreshFeed(userID, feedID int64) error {
	defer helper.ExecutionTime(time.Now(), fmt.Sprintf("[Handler:RefreshFeed] feedID=%d", feedID))

	originalFeed, err := h.store.FeedByID(userID, feedID)
	if err != nil {
		return err
	}

	if originalFeed == nil {
		return errors.NewLocalizedError(errNotFound, feedID)
	}

	client := http.NewClientWithCacheHeaders(originalFeed.FeedURL, originalFeed.EtagHeader, originalFeed.LastModifiedHeader)
	response, err := client.Get()
	if err != nil {
		customErr := errors.NewLocalizedError(errRequestFailed, err)
		originalFeed.ParsingErrorCount++
		originalFeed.ParsingErrorMsg = customErr.Error()
		h.store.UpdateFeed(originalFeed)
		return customErr
	}

	originalFeed.CheckedAt = time.Now()

	if response.HasServerFailure() {
		err := errors.NewLocalizedError(errServerFailure, response.StatusCode)
		originalFeed.ParsingErrorCount++
		originalFeed.ParsingErrorMsg = err.Error()
		h.store.UpdateFeed(originalFeed)
		return err
	}

	if response.IsModified(originalFeed.EtagHeader, originalFeed.LastModifiedHeader) {
		log.Printf("[Handler:RefreshFeed] Feed #%d has been modified\n", feedID)
		body, err := response.NormalizeBodyEncoding()
		if err != nil {
			return errors.NewLocalizedError(errEncoding, err)
		}

		subscription, err := parseFeed(body)
		if err != nil {
			originalFeed.ParsingErrorCount++
			originalFeed.ParsingErrorMsg = err.Error()
			h.store.UpdateFeed(originalFeed)
			return err
		}

		feedProcessor := processor.NewFeedProcessor(subscription)
		feedProcessor.WithScraperRules(originalFeed.ScraperRules)
		feedProcessor.WithRewriteRules(originalFeed.RewriteRules)
		feedProcessor.WithCrawler(originalFeed.Crawler)
		feedProcessor.Process()

		originalFeed.EtagHeader = response.ETag
		originalFeed.LastModifiedHeader = response.LastModified

		if err := h.store.UpdateEntries(originalFeed.UserID, originalFeed.ID, subscription.Entries); err != nil {
			return err
		}

		if !h.store.HasIcon(originalFeed.ID) {
			log.Println("[Handler:RefreshFeed] Looking for feed icon")
			icon, err := icon.FindIcon(originalFeed.SiteURL)
			if err != nil {
				log.Println("[Handler:RefreshFeed]", err)
			} else {
				h.store.CreateFeedIcon(originalFeed, icon)
			}
		}
	} else {
		log.Printf("[Handler:RefreshFeed] Feed #%d not modified\n", feedID)
	}

	originalFeed.ParsingErrorCount = 0
	originalFeed.ParsingErrorMsg = ""

	return h.store.UpdateFeed(originalFeed)
}

// NewFeedHandler returns a feed handler.
func NewFeedHandler(store *storage.Storage) *Handler {
	return &Handler{store: store}
}
