// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/organization"
	packages_model "github.com/neko233-com/gitsss/models/packages"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/structs"
	org_service "github.com/neko233-com/gitsss/services/org"
)

// SafeDeleteUser removes memberships without deleting owned assets or organizations.
func SafeDeleteUser(ctx context.Context, u *user_model.User) error {
	if u.IsOrganization() {
		return fmt.Errorf("%s is an organization not a user", u.Name)
	}
	if u.IsActive && user_model.IsLastAdminUser(ctx, u) {
		return user_model.ErrDeleteLastAdminUser{UID: u.ID}
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		count, err := repo_model.CountRepositories(ctx, repo_model.CountRepositoryOptions{OwnerID: u.ID})
		if err != nil {
			return err
		}
		if count > 0 {
			return repo_model.ErrUserOwnRepos{UID: u.ID}
		}
		ownsPackages, err := db.GetEngine(ctx).Where("owner_id = ?", u.ID).Exist(new(packages_model.Package))
		if err != nil {
			return err
		}
		if ownsPackages {
			return packages_model.ErrUserOwnPackages{UID: u.ID}
		}
		orgs, err := db.Find[organization.Organization](ctx, organization.FindOrgOptions{
			ListOptions: db.ListOptionsAll, UserID: u.ID, IncludeVisibility: structs.VisibleTypePrivate,
		})
		if err != nil {
			return err
		}
		for _, org := range orgs {
			if err := org_service.RemoveOrgUser(ctx, org, u); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// DeleteUser repeats ownership checks after memberships are committed.
	return DeleteUser(ctx, u, false)
}
