// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"

	"github.com/neko233-com/gitsss/models/organization"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/softwarecenter"
)

// RequireSoftwareMember matches the private web software center, including restricted accounts.
func RequireSoftwareMember(ctx *context.APIContext) {
	if ctx.PublicOnly {
		ctx.APIError(http.StatusForbidden, "Software center requires a token with private organization access")
		return
	}
	member, err := organization.IsOrganizationMember(ctx, ctx.Org.Organization.ID, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if !member {
		ctx.APIErrorNotFound()
		return
	}
	if ctx.Doer.IsRestricted {
		owner, err := ctx.Org.Organization.IsOwnedBy(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if !owner {
			ctx.APIErrorNotFound()
		}
	}
}

func softwareError(ctx *context.APIContext, err error) {
	switch {
	case errors.Is(err, softwarecenter.ErrInvalid):
		ctx.APIError(http.StatusBadRequest, err.Error())
	case errors.Is(err, softwarecenter.ErrConflict):
		ctx.APIError(http.StatusConflict, err.Error())
	case errors.Is(err, softwarecenter.ErrQuota):
		ctx.APIError(http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, softwarecenter.ErrBusy):
		ctx.APIError(http.StatusTooManyRequests, err.Error())
	default:
		ctx.APIErrorInternal(err)
	}
}

func ListSoftware(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/software organization orgListSoftware
	// ---
	// summary: List private organization software, directories first then name A-Z
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   type: string
	//   required: true
	// - name: parent
	//   in: query
	//   type: string
	// - name: q
	//   in: query
	//   type: string
	// - name: platform
	//   in: query
	//   type: string
	//   description: Filter files by Windows, macOS, iOS, Android, all, or unknown. Directories remain visible for navigation.
	// - name: recursive
	//   in: query
	//   type: boolean
	// - name: page
	//   in: query
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/SoftwareListing"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "500":
	//     "$ref": "#/responses/error"
	listing, err := softwarecenter.ListFiltered(ctx, ctx.Org.Organization.ID, ctx.FormString("parent"), ctx.FormTrim("q"), ctx.FormInt("page"), ctx.FormBool("recursive"), ctx.FormTrim("platform"))
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.Resp.Header().Set("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, listing)
}

func OrganizeSoftware(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/software/organize organization orgOrganizeSoftware
	// ---
	// summary: Preview or atomically apply suffix-based software platform organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/SoftwareOrganizeOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/SoftwareOrganization"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"
	//   "413":
	//     "$ref": "#/responses/error"
	//   "500":
	//     "$ref": "#/responses/error"
	report, err := softwarecenter.Organize(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, *web.GetForm(ctx).(*softwarecenter.OrganizeOptions))
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.Resp.Header().Set("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, report)
}
