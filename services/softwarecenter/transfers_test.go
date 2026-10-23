// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package softwarecenter

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func transferFixture(t *testing.T) {
	t.Helper()
	require.NoError(t, unittest.PrepareTestDatabase())
	data, repos := setting.AppDataPath, setting.RepoRootPath
	setting.AppDataPath, setting.RepoRootPath = t.TempDir(), filepath.Join(t.TempDir(), "repositories")
	t.Cleanup(func() { setting.AppDataPath, setting.RepoRootPath = data, repos })
	snapshotLock.Lock()
	delete(snapshots, 3)
	snapshotLock.Unlock()
}
func digestBytes(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }

func TestResumableConcurrentUsersAndTabs(t *testing.T) {
	transferFixture(t)
	release := make(chan struct{})
	type pending struct {
		user    int64
		session *UploadStatus
		reader  *blockedUploadReader
	}
	uploads := []pending{}
	for _, user := range []int64{2, 4} {
		for tab := range 2 {
			name := fmt.Sprintf("user-%d-tab-%d.zip", user, tab)
			session, err := BeginUpload(t.Context(), 3, user, "", name, digestBytes([]byte(name)), 4)
			require.NoError(t, err)
			uploads = append(uploads, pending{user, session, &blockedUploadReader{ready: make(chan struct{}), release: release, source: strings.NewReader("data")}})
		}
	}
	var workers sync.WaitGroup
	for _, upload := range uploads {
		workers.Go(func() {
			_, err := AppendUpload(t.Context(), 3, upload.user, upload.session.ID, 0, 4, digestBytes([]byte("data")), upload.reader)
			assert.NoError(t, err)
		})
	}
	for _, upload := range uploads {
		<-upload.reader.ready
	}
	close(release)
	workers.Wait()
	for _, user := range []int64{2, 4} {
		own, err := Uploads(t.Context(), 3, user)
		require.NoError(t, err)
		assert.Len(t, own, 2)
		for _, session := range own {
			assert.EqualValues(t, 4, session.Offset)
			node, err := CompleteUpload(t.Context(), 3, user, session.ID)
			require.NoError(t, err)
			assert.Equal(t, digestBytes([]byte("data")), node.Digest)
		}
	}
}

func TestResumableUploadDurableOffsetsAndIntegrity(t *testing.T) {
	transferFixture(t)
	ctx := t.Context()
	payload := bytes.Repeat([]byte("abcdefgh"), int(ChunkSize/8)+3)
	session, err := BeginUpload(ctx, 3, 2, "", "large.bin", digestBytes(payload), int64(len(payload)))
	require.NoError(t, err)
	same, err := BeginUpload(ctx, 3, 2, "", "large.bin", digestBytes(payload), int64(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, session.ID, same.ID)
	_, err = GetUpload(ctx, 3, 1, session.ID)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = AppendUpload(ctx, 3, 1, session.ID, 0, ChunkSize, digestBytes(payload[:ChunkSize]), bytes.NewReader(payload[:ChunkSize]))
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = CompleteUpload(ctx, 3, 2, session.ID)
	require.ErrorIs(t, err, ErrConflict)
	_, err = AppendUpload(ctx, 3, 2, session.ID, 0, ChunkSize, digestBytes([]byte("wrong")), bytes.NewReader(payload[:ChunkSize]))
	require.ErrorIs(t, err, ErrInvalid)
	part := filepath.Join(root(3), session.ID+".part")
	info, err := os.Stat(part)
	require.NoError(t, err)
	assert.Zero(t, info.Size())
	_, err = AppendUpload(ctx, 3, 2, session.ID, 0, ChunkSize, digestBytes(payload[:ChunkSize]), bytes.NewReader(payload[:100]))
	require.ErrorIs(t, err, ErrInvalid)
	session, err = AppendUpload(ctx, 3, 2, session.ID, 0, ChunkSize, digestBytes(payload[:ChunkSize]), bytes.NewReader(payload[:ChunkSize]))
	require.NoError(t, err)
	assert.Equal(t, ChunkSize, session.Offset)
	persisted, err := Load(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, ChunkSize, persisted.Uploads[session.ID].Offset)
	assert.NotEmpty(t, persisted.Uploads[session.ID].HashState)
	_, err = AppendUpload(ctx, 3, 2, session.ID, 0, ChunkSize, digestBytes(payload[:ChunkSize]), bytes.NewReader(payload[:ChunkSize]))
	require.ErrorIs(t, err, ErrConflict)
	// Simulate a process dying after writing a tail but before committing its offset.
	f, err := os.OpenFile(part, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString("uncommitted crash tail")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	remaining := payload[ChunkSize:]
	session, err = AppendUpload(ctx, 3, 2, session.ID, ChunkSize, int64(len(remaining)), digestBytes(remaining), bytes.NewReader(remaining))
	require.NoError(t, err)
	node, err := CompleteUpload(ctx, 3, 2, session.ID)
	require.NoError(t, err)
	assert.Equal(t, digestBytes(payload), node.Digest)
	require.NotNil(t, node.CRC32)
	assert.Equal(t, crc32.ChecksumIEEE(payload), *node.CRC32)
	repeated, err := CompleteUpload(ctx, 3, 2, session.ID)
	require.NoError(t, err)
	assert.Equal(t, node.ID, repeated.ID)
	completed, err := GetUpload(ctx, 3, 2, session.ID)
	require.NoError(t, err)
	require.NotNil(t, completed.Node)
	_, err = os.Stat(part)
	assert.True(t, os.IsNotExist(err))
	_, reader, err := Open(ctx, 3, node.ID)
	require.NoError(t, err)
	actual, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Equal(t, payload, actual)
	state, err := Load(ctx, 3)
	require.NoError(t, err)
	assert.Empty(t, state.Uploads)
	assert.Empty(t, state.Garbage)
}

func TestResumableCancelEmptyAndQuota(t *testing.T) {
	transferFixture(t)
	ctx := t.Context()
	require.NoError(t, SetQuota(ctx, 3, 10))
	session, err := BeginUpload(ctx, 3, 2, "", "pending", digestBytes(nil), 10)
	require.NoError(t, err)
	_, err = BeginUpload(ctx, 3, 2, "", "over", digestBytes(nil), 1)
	require.ErrorIs(t, err, ErrQuota)
	require.ErrorIs(t, CancelUpload(ctx, 3, 1, session.ID), os.ErrNotExist)
	require.NoError(t, CancelUpload(ctx, 3, 2, session.ID))
	_, err = os.Stat(filepath.Join(root(3), session.ID+".part"))
	assert.True(t, os.IsNotExist(err))
	empty, err := BeginUpload(ctx, 3, 2, "", "empty.txt", digestBytes(nil), 0)
	require.NoError(t, err)
	node, err := CompleteUpload(ctx, 3, 2, empty.ID)
	require.NoError(t, err)
	assert.Equal(t, digestBytes(nil), node.Digest)
	pending, err := BeginUpload(ctx, 3, 2, "", "expire", digestBytes(nil), 5)
	require.NoError(t, err)
	state, err := Load(ctx, 3)
	require.NoError(t, err)
	r := state.Uploads[pending.ID]
	r.Started = time.Now().Add(-8 * 24 * time.Hour).Unix()
	state.Uploads[pending.ID] = r
	prune(3, state)
	require.NoError(t, save(ctx, 3, state))
	assert.Empty(t, state.Uploads)
	_, err = os.Stat(filepath.Join(root(3), pending.ID+".part"))
	assert.True(t, os.IsNotExist(err))
}

func TestSoftwareBulkMoveDeleteAtomic(t *testing.T) {
	transferFixture(t)
	ctx := t.Context()
	dirs, err := EnsureDirectories(ctx, 3, 2, "", []string{"SDK/Go", "SDK/Java", "IDE", "Other/Go"})
	require.NoError(t, err)
	_, err = EnsureDirectories(ctx, 3, 2, "", []string{"Fresh", "../escape"})
	require.ErrorIs(t, err, ErrInvalid)
	state, err := Load(ctx, 3)
	require.NoError(t, err)
	for _, n := range state.Nodes {
		assert.NotEqual(t, "Fresh", n.Name)
	}
	assert.ErrorIs(t, Move(ctx, 3, []string{dirs["SDK"]}, dirs["SDK/Go"]), ErrInvalid)
	assert.ErrorIs(t, Move(ctx, 3, []string{dirs["SDK/Go"], dirs["Other/Go"]}, dirs["IDE"]), ErrConflict)
	require.NoError(t, Move(ctx, 3, []string{dirs["SDK"], dirs["SDK/Go"]}, dirs["IDE"]))
	state, err = Load(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, dirs["IDE"], state.Nodes[dirs["SDK"]].Parent)
	assert.Equal(t, dirs["SDK"], state.Nodes[dirs["SDK/Go"]].Parent)
	pending, err := BeginUpload(ctx, 3, 2, dirs["SDK/Go"], "busy", digestBytes(nil), 1)
	require.NoError(t, err)
	assert.ErrorIs(t, DeleteMany(ctx, 3, []string{dirs["IDE"], dirs["Other"]}), ErrBusy)
	assert.ErrorIs(t, Move(ctx, 3, []string{dirs["IDE"]}, ""), ErrBusy)
	require.NoError(t, CancelUpload(ctx, 3, 2, pending.ID))
	require.NoError(t, DeleteMany(ctx, 3, []string{dirs["IDE"], dirs["SDK"], dirs["Other"]}))
	state, err = Load(ctx, 3)
	require.NoError(t, err)
	assert.Empty(t, state.Nodes)
}

func TestVirtualZIP64ExtractionRangesAndTickets(t *testing.T) {
	transferFixture(t)
	ctx := t.Context()
	dirs, err := EnsureDirectories(ctx, 3, 2, "", []string{"工具/空目录"})
	require.NoError(t, err)
	payload := []byte("reproducible archive contents 中文")
	node, err := Upload(ctx, 3, 2, dirs["工具"], "工具.txt", int64(len(payload)), bytes.NewReader(payload))
	require.NoError(t, err)
	state, err := Load(ctx, 3)
	require.NoError(t, err)
	state.Nodes[node.ID].CRC32 = nil
	require.NoError(t, save(ctx, 3, state))
	job, err := CreateDownload(ctx, 3, 2, []string{dirs["工具"], node.ID})
	require.NoError(t, err)
	assert.False(t, job.Single)
	_, _, err = OpenDownload(ctx, 3, 1, job.ID)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, archive, err := OpenDownload(ctx, 3, 2, job.ID)
	require.NoError(t, err)
	data, err := io.ReadAll(archive)
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	assert.EqualValues(t, len(data), job.Size)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.Len(t, zr.File, 3)
	for _, entry := range zr.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		reader, err := entry.Open()
		require.NoError(t, err)
		actual, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		assert.Equal(t, payload, actual)
	}
	_, archive, err = OpenDownload(ctx, 3, 2, job.ID)
	require.NoError(t, err)
	defer archive.Close()
	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	req.Header.Set("Range", "bytes=17-95")
	req.Header.Set("If-Range", job.ETag)
	result := httptest.NewRecorder()
	result.Header().Set("ETag", job.ETag)
	http.ServeContent(result, req, job.Name, time.Unix(job.Created, 0), archive)
	assert.Equal(t, http.StatusPartialContent, result.Code)
	assert.Equal(t, data[17:96], result.Body.Bytes())
	update := StartDownloadProgress(job.ID, job.Size, 17)
	update(79, "paused")
	require.NoError(t, FinishDownload(ctx, 3, 2, job.ID))
	transferRecords.Delete(job.ID)
	persisted, err := GetDownload(ctx, 3, 2, job.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 96, DownloadProgress(persisted).Bytes)
	assert.Equal(t, "paused", DownloadProgress(persisted).State)
	require.NoError(t, Rename(ctx, 3, node.ID, "renamed.txt"))
	_, _, err = OpenDownload(ctx, 3, 2, job.ID)
	require.ErrorIs(t, err, ErrConflict)
	require.NoError(t, RemoveDownload(ctx, 3, 2, job.ID))
	_, err = GetDownload(ctx, 3, 2, job.ID)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestArchivePlanCacheServesIndependentReaders(t *testing.T) {
	transferFixture(t)
	ctx := t.Context()
	dir, err := CreateFolder(ctx, 3, 2, "", "SDK")
	require.NoError(t, err)
	for _, name := range []string{"a.bin", "b.bin"} {
		_, err = Upload(ctx, 3, 2, dir.ID, name, 4, strings.NewReader("data"))
		require.NoError(t, err)
	}
	job, err := CreateDownload(ctx, 3, 2, []string{dir.ID})
	require.NoError(t, err)
	assert.False(t, job.Single)
	_, first, err := OpenDownload(ctx, 3, 2, job.ID)
	require.NoError(t, err)
	_, second, err := OpenDownload(ctx, 3, 2, job.ID)
	require.NoError(t, err)
	head := make([]byte, 5)
	_, err = io.ReadFull(first, head)
	require.NoError(t, err)
	all, err := io.ReadAll(second)
	require.NoError(t, err)
	require.NoError(t, first.Close())
	require.NoError(t, second.Close())
	assert.Equal(t, head, all[:5])
	assert.Len(t, all, int(job.Size))
	_, err = EnsureDirectories(ctx, 3, 2, "", []string{"Unrelated"})
	require.NoError(t, err)
	_, third, err := OpenDownload(ctx, 3, 2, job.ID)
	require.NoError(t, err)
	reused, err := io.ReadAll(third)
	require.NoError(t, err)
	require.NoError(t, third.Close())
	assert.Equal(t, all, reused)
	require.NoError(t, Rename(ctx, 3, dir.ID, "SDK2"))
	_, _, err = OpenDownload(ctx, 3, 2, job.ID)
	require.ErrorIs(t, err, ErrConflict)
}

func TestVirtualZIPRejectUnsafePathsAndLargeMetadata(t *testing.T) {
	transferFixture(t)
	for _, name := range []string{"../escape", "/absolute", "C:/drive", "a\\b", "a//b", "a/./b"} {
		_, err := newVirtualZIP(3, []ArchiveEntry{{Path: name, Digest: digestBytes(nil)}})
		assert.ErrorIs(t, err, ErrInvalid)
	}
	archive, err := newVirtualZIP(3, []ArchiveEntry{{Path: "large.bin", Digest: digestBytes(nil), Size: 5 * 1024 * 1024 * 1024}})
	require.NoError(t, err)
	defer archive.Close()
	reader, err := zip.NewReader(zeroBlobReader{archive}, archive.Size())
	require.NoError(t, err)
	assert.EqualValues(t, 5*1024*1024*1024, reader.File[0].UncompressedSize64)
	_, err = archive.Seek(-1, io.SeekStart)
	assert.ErrorIs(t, err, ErrInvalid)
	_, err = archive.Seek(0, io.SeekEnd)
	require.NoError(t, err)
	buffer := make([]byte, 1)
	_, err = archive.Read(buffer)
	assert.ErrorIs(t, err, io.EOF)
	// A virtual archive stores headers only, never a second copy of file bytes.
	total := 0
	for _, segment := range archive.segments {
		total += len(segment.data)
	}
	assert.Less(t, total, 1024)
	_, err = newVirtualZIP(3, []ArchiveEntry{{Path: strings.Repeat("x", 65536), Directory: true}})
	assert.ErrorIs(t, err, ErrInvalid)
}

type zeroBlobReader struct{ v *virtualZIP }

func (r zeroBlobReader) ReadAt(p []byte, offset int64) (int, error) {
	total := 0
	for _, seg := range r.v.segments {
		if offset >= seg.offset+seg.size || offset < seg.offset {
			continue
		}
		n := int(min(int64(len(p)), seg.size-(offset-seg.offset)))
		if seg.data == nil {
			clear(p[:n])
		} else {
			copy(p[:n], seg.data[offset-seg.offset:offset-seg.offset+int64(n)])
		}
		total += n
		offset += int64(n)
		p = p[n:]
		if len(p) == 0 {
			return total, nil
		}
	}
	return total, io.EOF
}

func TestFolderNameIndexRetainsUnicodeCaseConflicts(t *testing.T) {
	assert.Equal(t, foldName("SDK"), foldName("ſdk"))
	assert.Equal(t, foldName("Kelvin"), foldName("Kelvin"))
	transferFixture(t)
	_, err := EnsureDirectories(t.Context(), 3, 2, "", []string{"SDK"})
	require.NoError(t, err)
	_, err = EnsureDirectories(t.Context(), 3, 2, "", []string{"ſdk"})
	require.ErrorIs(t, err, ErrConflict)
	_, err = EnsureDirectories(t.Context(), 3, 2, "", []string{strings.Repeat("x/", 32) + "y"})
	require.ErrorIs(t, err, ErrInvalid)
}
