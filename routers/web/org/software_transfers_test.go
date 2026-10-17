// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org_test

import (
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/models/organization"
	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/routers/web/org"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/contexttest"
	"github.com/neko233-com/gitsss/services/softwarecenter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSoftwareRepeatedFormFieldsParsedBeforeAccess(t *testing.T) {
	unittest.PrepareTestEnv(t)
	data, repos := setting.AppDataPath, setting.RepoRootPath
	setting.AppDataPath, setting.RepoRootPath = t.TempDir(), filepath.Join(t.TempDir(), "repositories")
	t.Cleanup(func() { setting.AppDataPath, setting.RepoRootPath = data, repos })
	a, err := softwarecenter.CreateFolder(t.Context(), 3, 2, "", "A")
	require.NoError(t, err)
	b, err := softwarecenter.CreateFolder(t.Context(), 3, 2, "", "B")
	require.NoError(t, err)
	call := func(action string, form url.Values, handler func(*context.Context), expected int) {
		t.Helper()
		ctx, _ := contexttest.MockContext(t, "/org3/-/software")
		contexttest.LoadUser(t, ctx, 2)
		ctx.Org = &context.Organization{Organization: &organization.Organization{ID: 3}}
		ctx.Req.Form, ctx.Req.PostForm = nil, nil
		ctx.Req.Method = http.MethodPost
		ctx.Req.Body = io.NopCloser(strings.NewReader(form.Encode()))
		ctx.Req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		ctx.SetPathParam("operation", action)
		handler(ctx)
		assert.Equal(t, expected, ctx.Resp.WrittenStatus())
	}
	call("move", url.Values{"ids": {a.ID}, "parent": {b.ID}}, org.SoftwareBatch, http.StatusOK)
	call("directories", url.Values{"paths": {"C/one", "C/two"}}, org.SoftwareBatch, http.StatusCreated)
	call("", url.Values{"ids": {b.ID}}, org.SoftwareDownloadBegin, http.StatusCreated)
	call("delete", url.Values{"ids": {a.ID, b.ID}}, org.SoftwareBatch, http.StatusOK)
}
