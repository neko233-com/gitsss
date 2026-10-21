// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"reflect"
	"testing"

	"github.com/neko233-com/gitsss/modules/hookrelay"
	"github.com/neko233-com/gitsss/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookRelayWireCompatibility(t *testing.T) {
	original := reflect.TypeFor[HookOptions]()
	relay := reflect.TypeFor[hookrelay.Options]()
	require.Equal(t, original.NumField(), relay.NumField())
	for index := range original.NumField() {
		assert.Equal(t, original.Field(index).Name, relay.Field(index).Name)
	}
	data, err := json.Marshal(hookrelay.Options{
		OldCommitIDs: []string{"old"}, NewCommitIDs: []string{"new"}, RefFullNames: []string{"refs/heads/main"},
		UserID: 17, UserName: "fixture", GitObjectDirectory: "objects/incoming-fixture", GitAlternativeObjectDirectories: "objects",
		GitQuarantinePath: "objects/incoming-fixture", GitPushOptions: map[string]string{"key": "value"},
		PullRequestID: 23, PushTrigger: "pr-merge-to-base", DeployKeyID: 29, IsWiki: true, ActionsTaskID: 31,
	})
	require.NoError(t, err)
	var decoded HookOptions
	require.NoError(t, json.Unmarshal(data, &decoded))
	encoded, err := json.Marshal(decoded)
	require.NoError(t, err)
	assert.JSONEq(t, string(data), string(encoded))
}
