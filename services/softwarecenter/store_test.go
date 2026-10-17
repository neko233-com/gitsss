// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package softwarecenter

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "github.com/neko233-com/gitsss/models"
	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/repolayout"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { unittest.MainTest(m) }

func TestSoftwareCenterTreeQuotaAndDedup(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	originalData, originalRepos := setting.AppDataPath, setting.RepoRootPath
	setting.AppDataPath = t.TempDir()
	setting.RepoRootPath = filepath.Join(t.TempDir(), "repositories")
	t.Cleanup(func() { setting.AppDataPath = originalData; setting.RepoRootPath = originalRepos })
	folder, err := CreateFolder(t.Context(), 3, 2, "", "SDK")
	require.NoError(t, err)
	nested, err := CreateFolder(t.Context(), 3, 2, folder.ID, "Language")
	require.NoError(t, err)
	_, err = CreateFolder(t.Context(), 3, 2, "", "sdk")
	require.ErrorIs(t, err, ErrConflict)
	_, err = CreateFolder(t.Context(), 3, 2, "", "../outside")
	require.ErrorIs(t, err, ErrInvalid)
	a, err := Upload(t.Context(), 3, 2, nested.ID, "tool.zip", 4, strings.NewReader("data"))
	require.NoError(t, err)
	b, err := Upload(t.Context(), 3, 2, "", "copy.zip", 4, strings.NewReader("data"))
	require.NoError(t, err)
	assert.Equal(t, a.Digest, b.Digest)
	files, err := os.ReadDir(filepath.Join(root(3), "blobs"))
	require.NoError(t, err)
	assert.Len(t, files, 1)
	require.NoError(t, Rename(t.Context(), 3, folder.ID, "Tools"))
	loaded, file, err := Open(t.Context(), 3, a.ID)
	require.NoError(t, err)
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	assert.Equal(t, []byte("data"), content)
	assert.Equal(t, "tool.zip", loaded.Name)
	require.NoError(t, Delete(t.Context(), 3, folder.ID))
	_, file, err = Open(t.Context(), 3, b.ID)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	s, err := Load(t.Context(), 3)
	require.NoError(t, err)
	assert.EqualValues(t, 4, s.Used())
	assert.Len(t, s.Nodes, 1)
	s.Quota = 5
	require.NoError(t, save(t.Context(), 3, s))
	_, err = Upload(t.Context(), 3, 2, "", "too-big.bin", 2, bytes.NewReader([]byte{1, 2}))
	require.ErrorIs(t, err, ErrQuota)
	_, err = Upload(t.Context(), 3, 2, "", "broken.bin", 1, strings.NewReader("xx"))
	require.ErrorIs(t, err, ErrInvalid)
	s, err = Load(t.Context(), 3)
	require.NoError(t, err)
	assert.Empty(t, s.Uploads)
	assert.ErrorIs(t, SetQuota(t.Context(), 3, 3), ErrQuota)
	require.NoError(t, Delete(t.Context(), 3, b.ID))
	files, err = os.ReadDir(filepath.Join(root(3), "blobs"))
	require.NoError(t, err)
	assert.Empty(t, files)
	manifest, err := repolayout.Read(setting.AppDataPath)
	require.NoError(t, err)
	assert.Equal(t, "gitsss/extensions/software-center", manifest.Reserved["software-center"])
}

type blockedUploadReader struct {
	once    sync.Once
	ready   chan struct{}
	release chan struct{}
	source  io.Reader
}

func (r *blockedUploadReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.ready); <-r.release })
	return r.source.Read(p)
}

func TestSoftwareCenterReservationsAndSnapshotIsolation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	data, repos := setting.AppDataPath, setting.RepoRootPath
	setting.AppDataPath = t.TempDir()
	setting.RepoRootPath = filepath.Join(t.TempDir(), "repositories")
	t.Cleanup(func() { setting.AppDataPath = data; setting.RepoRootPath = repos })
	folder, err := CreateFolder(t.Context(), 4, 2, "", "before")
	require.NoError(t, err)
	before, err := Snapshot(t.Context(), 4)
	require.NoError(t, err)
	require.NoError(t, Rename(t.Context(), 4, folder.ID, "after"))
	after, err := Snapshot(t.Context(), 4)
	require.NoError(t, err)
	assert.Equal(t, "before", before.Nodes[folder.ID].Name)
	assert.Equal(t, "after", after.Nodes[folder.ID].Name)
	require.NoError(t, SetQuota(t.Context(), 4, 4))
	reader := &blockedUploadReader{ready: make(chan struct{}), release: make(chan struct{}), source: strings.NewReader("data")}
	finished := make(chan error, 1)
	go func() {
		_, uploadErr := Upload(t.Context(), 4, 2, folder.ID, "data.bin", 4, reader)
		finished <- uploadErr
	}()
	<-reader.ready
	s, err := Load(t.Context(), 4)
	require.NoError(t, err)
	assert.EqualValues(t, 4, s.Reserved())
	_, err = Upload(t.Context(), 4, 2, "", "extra.bin", 1, strings.NewReader("x"))
	assert.ErrorIs(t, err, ErrQuota)
	assert.ErrorIs(t, Delete(t.Context(), 4, folder.ID), ErrBusy)
	close(reader.release)
	require.NoError(t, <-finished)
	require.NoError(t, Delete(t.Context(), 4, folder.ID))
	s, err = Load(t.Context(), 4)
	require.NoError(t, err)
	assert.Zero(t, s.Used())
	assert.Empty(t, s.Uploads)
	assert.Empty(t, s.Garbage)
}

func TestSoftwareCenterBoundaries(t *testing.T) {
	for _, name := range []string{"", "..", ".", "bad/name", "bad\\name", " trailing", "trailing.", "bad\nname"} {
		assert.False(t, validName(name), name)
	}
	assert.True(t, validName("IDE 2026.zip"))
	s := &State{Nodes: map[string]*Node{}, Uploads: map[string]Reservation{}, Quota: DefaultQuota}
	parent := ""
	for range 32 {
		node := &Node{ID: newID(), Parent: parent, Name: "folder", Directory: true}
		s.Nodes[node.ID] = node
		parent = node.ID
	}
	require.ErrorIs(t, s.checkName(parent, "another", ""), ErrInvalid)
}
