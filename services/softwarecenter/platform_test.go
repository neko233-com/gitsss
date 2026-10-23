// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package softwarecenter

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSoftwarePlatformSuffix(t *testing.T) {
	for name, platform := range map[string]string{
		"tool.MSI": "Windows", "mac-name.exe": "Windows", "tool.msixbundle": "Windows",
		"tool.dmg": "macOS", "tool.pkg": "macOS", "tool.ipa": "iOS",
		"tool.apk": "Android", "tool.aab": "Android", "tool.xapk": "Android",
		"windows.zip": "all", "tool.7z": "all", "tool.rar": "all", "tool.tar.gz": "all",
		"tool.tar.xz": "all", "tool.tar.zst": "all", "tool": "unknown", "tool.msi.txt": "unknown",
	} {
		t.Run(name, func(t *testing.T) { assert.Equal(t, platform, Platform(name)) })
	}
}

func TestSoftwareListFiltersPlatformAndQuery(t *testing.T) {
	organizeFixture(t)
	listing, err := ListFiltered(t.Context(), 3, "vpn", "", 1, false, "windows")
	require.NoError(t, err)
	require.Len(t, listing.Entries, 1)
	assert.Equal(t, "a.MSI", listing.Entries[0].Name)
	assert.Equal(t, "Windows", listing.Entries[0].Platform)

	listing, err = ListFiltered(t.Context(), 3, "vpn", "B", 1, false, "Windows")
	require.NoError(t, err)
	assert.Empty(t, listing.Entries)

	listing, err = ListFiltered(t.Context(), 3, "", "", 1, false, "all")
	require.NoError(t, err)
	var directories, files int
	for _, entry := range listing.Entries {
		if entry.Directory {
			directories++
		} else {
			files++
			assert.Equal(t, "all", entry.Platform)
		}
	}
	assert.Equal(t, 2, directories, "directories stay visible for navigation")
	assert.Equal(t, 1, files)

	_, err = ListFiltered(t.Context(), 3, "", "", 1, false, "desktop")
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestSoftwareOrganizeEmptySubtreeDoesNotCreatePlatformFolders(t *testing.T) {
	s := organizeFixture(t)
	s.Nodes = map[string]*Node{}
	require.NoError(t, save(t.Context(), 3, s))

	preview, err := Organize(t.Context(), 3, 2, OrganizeOptions{})
	require.NoError(t, err)
	assert.Empty(t, preview.Moves)
	assert.Empty(t, preview.Created)
	assert.Empty(t, preview.Skipped)
	assert.False(t, preview.Applied)
}

func organizeFixture(t *testing.T) *State {
	t.Helper()
	require.NoError(t, unittest.PrepareTestDatabase())
	s, err := Load(t.Context(), 3)
	require.NoError(t, err)
	for _, node := range []*Node{
		{ID: "vpn", Name: "VPN", Directory: true},
		{ID: "empty", Name: "SDK", Directory: true},
		{ID: "win", Parent: "vpn", Name: "a.MSI", Digest: "original-win", Size: 23},
		{ID: "mac", Parent: "vpn", Name: "B.dmg", Digest: "original-mac", Size: 42},
		{ID: "zip", Name: "c-windows.zip", Digest: "original-zip", Size: 10},
		{ID: "unknown", Name: "readme.txt"},
	} {
		s.Nodes[node.ID] = node
	}
	require.NoError(t, save(t.Context(), 3, s))
	return s
}

func TestSoftwareOrganizePreviewAtomicApplyAndRepeat(t *testing.T) {
	before := organizeFixture(t)
	preview, err := Organize(t.Context(), 3, 2, OrganizeOptions{})
	require.NoError(t, err)
	assert.False(t, preview.Applied)
	assert.Len(t, preview.Skipped, 1)
	loaded, err := Load(t.Context(), 3)
	require.NoError(t, err)
	assert.Equal(t, before, loaded)
	_, err = Organize(t.Context(), 3, 2, OrganizeOptions{Apply: true})
	require.ErrorIs(t, err, ErrConflict)
	result, err := Organize(t.Context(), 3, 2, OrganizeOptions{Apply: true, Revision: preview.Revision})
	require.NoError(t, err)
	assert.True(t, result.Applied)
	loaded, err = Load(t.Context(), 3)
	require.NoError(t, err)
	for id, expected := range map[string]string{"win": "Windows/VPN/a.MSI", "mac": "macOS/VPN/B.dmg", "zip": "全平台/c-windows.zip", "empty": "全平台/SDK", "unknown": "readme.txt"} {
		assert.Equal(t, expected, nodePath(loaded, id, ""))
		assert.Equal(t, before.Nodes[id].Digest, loaded.Nodes[id].Digest)
		assert.Equal(t, before.Nodes[id].Size, loaded.Nodes[id].Size)
	}
	assert.Equal(t, before.Used(), loaded.Used())
	repeat, err := Organize(t.Context(), 3, 2, OrganizeOptions{})
	require.NoError(t, err)
	assert.Empty(t, repeat.Moves)
	assert.Empty(t, repeat.Created)
	_, err = Organize(t.Context(), 3, 2, OrganizeOptions{Apply: true, Revision: preview.Revision})
	require.ErrorIs(t, err, ErrConflict)
	listing, err := List(t.Context(), 3, "", "", 1, true)
	require.NoError(t, err)
	for _, entry := range listing.Entries {
		if entry.ID == "win" {
			assert.Equal(t, "Windows", entry.Platform)
			assert.Equal(t, "Windows/VPN/a.MSI", entry.Path)
		}
	}
}

func TestSoftwareOrganizeConflictsDoNotWrite(t *testing.T) {
	for _, scenario := range []string{"collision", "pending-name", "depth", "invalid-override"} {
		t.Run(scenario, func(t *testing.T) {
			s := organizeFixture(t)
			switch scenario {
			case "collision":
				s.Nodes["block"] = &Node{ID: "block", Name: "Windows"}
			case "pending-name":
				s.Uploads["pending"] = Reservation{Name: "Windows", Started: time.Now().Unix()}
			case "depth":
				parent := "vpn"
				for i := range 30 {
					id := string(rune('a' + i))
					s.Nodes[id] = &Node{ID: id, Parent: parent, Name: id, Directory: true}
					parent = id
				}
				s.Nodes["win"].Parent = parent
			}
			require.NoError(t, save(t.Context(), 3, s))
			opts := OrganizeOptions{}
			if scenario == "invalid-override" {
				opts.Platforms = map[string]string{"not-in-this-org": "Windows"}
			}
			_, err := Organize(t.Context(), 3, 2, opts)
			require.Error(t, err)
			after, err := Load(t.Context(), 3)
			require.NoError(t, err)
			assert.Equal(t, s, after)
		})
	}
}

func TestSoftwareOrganizeActiveUploadAndOverride(t *testing.T) {
	s := organizeFixture(t)
	s.Uploads["pending"] = Reservation{Parent: "vpn", Name: "pending.exe", Started: time.Now().Unix()}
	require.NoError(t, save(t.Context(), 3, s))
	opts := OrganizeOptions{Platforms: map[string]string{"unknown": "all"}}
	preview, err := Organize(t.Context(), 3, 2, opts)
	require.NoError(t, err)
	opts.Apply, opts.Revision = true, preview.Revision
	_, err = Organize(t.Context(), 3, 2, opts)
	require.NoError(t, err)
	after, err := Load(t.Context(), 3)
	require.NoError(t, err)
	assert.Empty(t, after.Nodes["vpn"].Parent)
	assert.Equal(t, s.Uploads, after.Uploads)
	assert.Equal(t, "全平台/readme.txt", nodePath(after, "unknown", ""))
}

func TestSoftwareOrganizeRevisionIgnoresTransferProgress(t *testing.T) {
	s := organizeFixture(t)
	s.Uploads["pending"] = Reservation{Parent: "vpn", Name: "pending.exe", Size: 10, Resumable: true, Chain: "first", Started: time.Now().Unix()}
	require.NoError(t, save(t.Context(), 3, s))
	preview, err := Organize(t.Context(), 3, 2, OrganizeOptions{})
	require.NoError(t, err)
	loaded, err := Load(t.Context(), 3)
	require.NoError(t, err)
	upload := loaded.Uploads["pending"]
	upload.Offset, upload.Started, upload.Chain, upload.HashState = 5, upload.Started+60, "second", []byte("progress")
	loaded.Uploads["pending"] = upload
	require.NoError(t, save(t.Context(), 3, loaded))
	report, err := Organize(t.Context(), 3, 2, OrganizeOptions{Apply: true, Revision: preview.Revision})
	require.NoError(t, err)
	assert.True(t, report.Applied)
}

func TestSoftwareListPaginatesAndHidesPrivateFields(t *testing.T) {
	transferFixture(t)
	ctx := t.Context()
	dir, err := CreateFolder(ctx, 3, 2, "", "SDK")
	require.NoError(t, err)
	state, err := Load(ctx, 3)
	require.NoError(t, err)
	for i := range 205 {
		id := fmt.Sprintf("%032x", i+1)
		state.Nodes[id] = &Node{ID: id, Parent: dir.ID, Name: fmt.Sprintf("f%03d.bin", i), Size: 1, Digest: strings.Repeat("a", 64), UserID: 9, UploadKey: "secret"}
	}
	require.NoError(t, save(ctx, 3, state))
	listing, err := List(ctx, 3, dir.ID, "", 2, false)
	require.NoError(t, err)
	assert.Equal(t, 205, listing.Total)
	assert.Equal(t, 2, listing.Page)
	require.Len(t, listing.Entries, 5)
	assert.Equal(t, "f200.bin", listing.Entries[0].Name)
	assert.Equal(t, "SDK/f200.bin", listing.Entries[0].Path)
	raw, err := json.Marshal(listing)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "upload_key")
	assert.NotContains(t, string(raw), `"user_id"`)
}
