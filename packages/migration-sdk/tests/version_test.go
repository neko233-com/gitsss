// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerVersion(t *testing.T) {
	log.Printf("== TestServerVersion ==")
	c := newTestClient()
	rawVersion, _, err := c.Meta.ServerVersion(t.Context())
	assert.NoError(t, err)
	assert.NotEmpty(t, rawVersion)
}
