// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import "github.com/neko233-com/gitsss/services/softwarecenter"

// swagger:parameters orgOrganizeSoftware
type swaggerSoftwareOrganizeOptions struct {
	// in:path
	// required: true
	Org string `json:"org"`
	// in:body
	Body softwarecenter.OrganizeOptions `json:"body"`
}

// SoftwareListing
// swagger:response SoftwareListing
type swaggerResponseSoftwareListing struct {
	// in:body
	Body softwarecenter.Listing
}

// SoftwareOrganization
// swagger:response SoftwareOrganization
type swaggerResponseSoftwareOrganization struct {
	// in:body
	Body softwarecenter.OrganizeReport
}
