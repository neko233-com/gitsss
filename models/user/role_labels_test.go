// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRoleLabels(t *testing.T) {
	labels, err := ParseRoleLabels(" 程序， 美术,策划,产品经理,程序, Dev,dev ")
	require.NoError(t, err)
	assert.Equal(t, []string{"程序", "美术", "策划", "产品经理", "Dev"}, labels)
	labels, err = ParseRoleLabels("")
	require.NoError(t, err)
	assert.Empty(t, labels)
	_, err = ParseRoleLabels(strings.Repeat("人", 33))
	assert.Error(t, err)
	_, err = ParseRoleLabels("bad\trole")
	assert.Error(t, err)
	_, err = ParseRoleLabels("a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u")
	assert.Error(t, err)
}
