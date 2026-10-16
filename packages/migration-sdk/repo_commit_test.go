// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryEncode(t *testing.T) {
	log.Println("== TestQueryEncode ==")
	opts := ListCommitOptions{}
	assert.EqualValues(t, "", opts.SHA)
	assert.EqualValues(t, "", opts.Path)
	assert.EqualValues(t, "", opts.Not)
	assert.EqualValues(t, false, opts.Stat)
	assert.EqualValues(t, false, opts.Verification)
	assert.EqualValues(t, false, opts.Files)

	query := opts.QueryEncode()
	assert.EqualValues(t,
		"files=false&limit=0&page=0&stat=false&verification=false",
		query)

	opts.Path = "gitea"
	query = opts.QueryEncode()
	assert.EqualValues(t,
		"files=false&limit=0&page=0&path=gitea&stat=false&verification=false",
		query)

	opts.SHA = "main"
	query = opts.QueryEncode()
	assert.EqualValues(t,
		"files=false&limit=0&page=0&path=gitea&sha=main&stat=false&verification=false",
		query)

	opts.Verification = true
	query = opts.QueryEncode()
	assert.EqualValues(t,
		"files=false&limit=0&page=0&path=gitea&sha=main&stat=false&verification=true",
		query)

	opts.Stat = true
	query = opts.QueryEncode()
	assert.EqualValues(t,
		"files=false&limit=0&page=0&path=gitea&sha=main&stat=true&verification=true",
		query)

	opts.Files = true
	query = opts.QueryEncode()
	assert.EqualValues(t,
		"files=true&limit=0&page=0&path=gitea&sha=main&stat=true&verification=true",
		query)
}
