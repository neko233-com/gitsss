// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"bytes"
	"encoding/base64"
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestFileCreateUpdateGet(t *testing.T) {
	log.Println("== TestFileCRUD ==")
	c := newTestClient()

	repo, err := createTestRepo(t, "ChangeFiles", c)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	raw, _, err := c.Repositories.GetFile(t.Context(), repo.Owner.UserName, repo.Name, "main", "README.md")
	assert.NoError(t, err)
	assert.EqualValues(t, "IyBDaGFuZ2VGaWxlcwoKQSB0ZXN0IFJlcG86IENoYW5nZUZpbGVz", base64.StdEncoding.EncodeToString(raw))

	testFileName := "A+#&ä"
	newFile, _, err := c.Repositories.CreateFile(t.Context(), repo.Owner.UserName, repo.Name, testFileName, gitea.CreateFileOptions{
		FileOptions: gitea.FileOptions{
			Message: "create file " + testFileName,
		},
		Content: "ZmlsZUEK",
	})
	assert.NoError(t, err)
	raw, _, _ = c.Repositories.GetFile(t.Context(), repo.Owner.UserName, repo.Name, "main", testFileName)
	assert.EqualValues(t, "ZmlsZUEK", base64.StdEncoding.EncodeToString(raw))

	updatedFile, _, err := c.Repositories.UpdateFile(t.Context(), repo.Owner.UserName, repo.Name, testFileName, gitea.UpdateFileOptions{
		FileOptions: gitea.FileOptions{
			Message: "add a new line",
		},
		SHA:     newFile.Content.SHA,
		Content: "ZmlsZUEKCmFuZCBhIG5ldyBsaW5lCg==",
	})
	assert.NoError(t, err)
	assert.NotNil(t, updatedFile)

	file, _, err := c.Repositories.GetContents(t.Context(), repo.Owner.UserName, repo.Name, "main", testFileName)
	assert.NoError(t, err)
	assert.EqualValues(t, updatedFile.Content.SHA, file.SHA)
	assert.EqualValues(t, &updatedFile.Content.Content, &file.Content)

	_, err = c.Repositories.DeleteFile(t.Context(), repo.Owner.UserName, repo.Name, testFileName, gitea.DeleteFileOptions{
		FileOptions: gitea.FileOptions{
			Message: "Delete File " + testFileName,
		},
		SHA: updatedFile.Content.SHA,
	})
	assert.NoError(t, err)
	_, resp, err := c.Repositories.GetFile(t.Context(), repo.Owner.UserName, repo.Name, "main", testFileName)
	assert.Error(t, err)
	assert.EqualValues(t, "not found", err.Error())
	assert.EqualValues(t, 404, resp.StatusCode)

	licence, _, err := c.Repositories.GetContents(t.Context(), repo.Owner.UserName, repo.Name, "", "LICENSE")
	assert.NoError(t, err)
	licenceRaw, _, err := c.Repositories.GetFile(t.Context(), repo.Owner.UserName, repo.Name, "", "LICENSE")
	assert.NoError(t, err)
	testContent := "Tk9USElORyBJUyBIRVJFIEFOWU1PUkUKSUYgWU9VIExJS0UgVE8gRklORCBTT01FVEhJTkcKV0FJVCBGT1IgVEhFIEZVVFVSRQo="
	updatedFile, _, err = c.Repositories.UpdateFile(t.Context(), repo.Owner.UserName, repo.Name, "LICENSE", gitea.UpdateFileOptions{
		FileOptions: gitea.FileOptions{
			Message:       "Overwrite",
			BranchName:    "main",
			NewBranchName: "overwrite-a+/&licence",
		},
		SHA:     licence.SHA,
		Content: testContent,
	})
	assert.NoError(t, err)
	assert.NotNil(t, updatedFile)
	licenceRawNew, _, err := c.Repositories.GetFile(t.Context(), repo.Owner.UserName, repo.Name, "overwrite-a+/&licence", "LICENSE")
	assert.NoError(t, err)
	assert.NotNil(t, licence)
	assert.False(t, bytes.Equal(licenceRaw, licenceRawNew))
	assert.EqualValues(t, testContent, base64.StdEncoding.EncodeToString(licenceRawNew))

	// ListContents in root dir of default branch
	dir, resp, err := c.Repositories.ListContents(t.Context(), repo.Owner.UserName, repo.Name, "", "")
	assert.NoError(t, err)
	assert.Len(t, dir, 3)
	assert.NotNil(t, resp)

	// ListContents in not existing dir of default branch
	_, resp, err = c.Repositories.ListContents(t.Context(), repo.Owner.UserName, repo.Name, "", "/hehe/")
	assert.Error(t, err)
	assert.EqualValues(t, 404, resp.StatusCode)
	// ListContents in root dir of not existing branch
	_, resp, err = c.Repositories.ListContents(t.Context(), repo.Owner.UserName, repo.Name, "no-ref-at-all", "")
	assert.Error(t, err)
	assert.EqualValues(t, 404, resp.StatusCode)

	// ListContents try to get file as dir
	dir, resp, err = c.Repositories.ListContents(t.Context(), repo.Owner.UserName, repo.Name, "", "LICENSE")
	if assert.Error(t, err) {
		assert.EqualValues(t, "expect directory, got file", err.Error())
	}
	assert.Nil(t, dir)
	assert.EqualValues(t, 200, resp.StatusCode)

	// GetContents try to get dir as file
	file, resp, err = c.Repositories.GetContents(t.Context(), repo.Owner.UserName, repo.Name, "", "")
	if assert.Error(t, err) {
		assert.EqualValues(t, "expect file, got directory", err.Error())
	}
	assert.Nil(t, file)
	assert.EqualValues(t, 200, resp.StatusCode)
}
