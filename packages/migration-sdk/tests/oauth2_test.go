// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestOauth2(t *testing.T) {
	log.Println("== TestOauth2Application ==")
	c := newTestClient()

	user := createTestUser(t, "oauth2_user", c)
	c.SetSudo(user.UserName)

	type test struct {
		name               string
		confidentialClient *bool
	}
	boolTrue := true
	boolFalse := false

	testCases := []test{
		{"ConfidentialClient unset should fallback to false", nil},
		{"ConfidentialClient true", &boolTrue},
		{"ConfidentialClient false", &boolFalse},
	}

	for _, testCase := range testCases {
		createOptions := gitea.CreateApplicationOption{
			Name:         "test",
			RedirectURIs: []string{"http://test/test"},
		}
		if testCase.confidentialClient != nil {
			createOptions.ConfidentialClient = *testCase.confidentialClient
		}

		newApp, _, err := c.OAuth2.CreateApplication(t.Context(), createOptions)
		assert.NoError(t, err, testCase.name)
		assert.NotNil(t, newApp, testCase.name)
		assert.EqualValues(t, "test", newApp.Name, testCase.name)
		if testCase.confidentialClient != nil {
			assert.EqualValues(t, *testCase.confidentialClient, newApp.ConfidentialClient, testCase.name)
		} else {
			assert.EqualValues(t, false, newApp.ConfidentialClient, testCase.name)
		}

		a, _, err := c.OAuth2.ListApplications(t.Context(), gitea.ListApplicationsOptions{})
		assert.NoError(t, err, testCase.name)
		assert.Len(t, a, 1, testCase.name)
		assert.EqualValues(t, newApp.Name, a[0].Name, testCase.name)
		assert.EqualValues(t, newApp.ConfidentialClient, a[0].ConfidentialClient, testCase.name)

		b, _, err := c.OAuth2.GetApplication(t.Context(), newApp.ID)
		assert.NoError(t, err, testCase.name)
		assert.EqualValues(t, newApp.Name, b.Name, testCase.name)
		assert.EqualValues(t, newApp.ConfidentialClient, b.ConfidentialClient, testCase.name)

		b, _, err = c.OAuth2.UpdateApplication(t.Context(), newApp.ID, gitea.CreateApplicationOption{
			Name:               newApp.Name,
			ConfidentialClient: !newApp.ConfidentialClient,
			RedirectURIs:       []string{"https://test/login"},
		})
		assert.NoError(t, err, testCase.name)
		assert.EqualValues(t, newApp.Name, b.Name, testCase.name)
		assert.EqualValues(t, "https://test/login", b.RedirectURIs[0], testCase.name)
		assert.EqualValues(t, newApp.ID, b.ID, testCase.name)
		assert.NotEqual(t, newApp.ClientSecret, b.ClientSecret, testCase.name)
		assert.NotEqual(t, newApp.ConfidentialClient, b.ConfidentialClient, testCase.name)

		_, err = c.OAuth2.DeleteApplication(t.Context(), newApp.ID)
		assert.NoError(t, err, testCase.name)
	}
}
