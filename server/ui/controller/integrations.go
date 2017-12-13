// Copyright 2017 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package controller

import (
	"crypto/md5"
	"fmt"

	"github.com/miniflux/miniflux/server/core"
	"github.com/miniflux/miniflux/server/ui/form"
)

// ShowIntegrations renders the page with all external integrations.
func (c *Controller) ShowIntegrations(ctx *core.Context, request *core.Request, response *core.Response) {
	user := ctx.LoggedUser()
	integration, err := c.store.Integration(user.ID)
	if err != nil {
		response.HTML().ServerError(err)
		return
	}

	args, err := c.getCommonTemplateArgs(ctx)
	if err != nil {
		response.HTML().ServerError(err)
		return
	}

	response.HTML().Render("integrations", args.Merge(tplParams{
		"menu": "settings",
		"form": form.IntegrationForm{
			PinboardEnabled:      integration.PinboardEnabled,
			PinboardToken:        integration.PinboardToken,
			PinboardTags:         integration.PinboardTags,
			PinboardMarkAsUnread: integration.PinboardMarkAsUnread,
			InstapaperEnabled:    integration.InstapaperEnabled,
			InstapaperUsername:   integration.InstapaperUsername,
			InstapaperPassword:   integration.InstapaperPassword,
			FeverEnabled:         integration.FeverEnabled,
			FeverUsername:        integration.FeverUsername,
			FeverPassword:        integration.FeverPassword,
		},
	}))
}

// UpdateIntegration updates integration settings.
func (c *Controller) UpdateIntegration(ctx *core.Context, request *core.Request, response *core.Response) {
	user := ctx.LoggedUser()
	integration, err := c.store.Integration(user.ID)
	if err != nil {
		response.HTML().ServerError(err)
		return
	}

	integrationForm := form.NewIntegrationForm(request.Request())
	integrationForm.Merge(integration)

	if integration.FeverEnabled {
		integration.FeverToken = fmt.Sprintf("%x", md5.Sum([]byte(integration.FeverUsername+":"+integration.FeverPassword)))
	} else {
		integration.FeverToken = ""
	}

	err = c.store.UpdateIntegration(integration)
	if err != nil {
		response.HTML().ServerError(err)
		return
	}

	response.Redirect(ctx.Route("integrations"))
}
