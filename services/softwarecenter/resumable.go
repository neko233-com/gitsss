// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package softwarecenter

import (
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/modules/repolayout"
	"github.com/neko233-com/gitsss/modules/setting"
)

const ChunkSize int64 = 8 * 1024 * 1024

var uploadLocks [256]sync.Mutex

func sessionLock(id string) *sync.Mutex {
	digest := sha256.Sum256([]byte(id))
	return &uploadLocks[digest[0]]
}

type UploadStatus struct {
	ID        string `json:"id"`
	Parent    string `json:"parent"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Offset    int64  `json:"offset"`
	ClientKey string `json:"client_key"`
	Chain     string `json:"chain"`
	ChunkSize int64  `json:"chunk_size"`
	Updated   int64  `json:"updated"`
	Node      *Node  `json:"node,omitempty"`
}

func uploadStatus(id string, r Reservation) *UploadStatus {
	return &UploadStatus{ID: id, Parent: r.Parent, Name: r.Name, Size: r.Size, Offset: r.Offset, ClientKey: r.ClientKey, Chain: r.Chain, ChunkSize: ChunkSize, Updated: r.Started}
}

func hexDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func Uploads(ctx context.Context, orgID, userID int64) ([]*UploadStatus, error) {
	s, err := Snapshot(ctx, orgID)
	if err != nil {
		return nil, err
	}
	items := []*UploadStatus{}
	for id, r := range s.Uploads {
		if r.Resumable && r.UserID == userID {
			items = append(items, uploadStatus(id, r))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Updated > items[j].Updated })
	return items, nil
}

func GetUpload(ctx context.Context, orgID, userID int64, id string) (*UploadStatus, error) {
	s, err := Snapshot(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if r, ok := s.Uploads[id]; ok && r.Resumable && r.UserID == userID {
		return uploadStatus(id, r), nil
	}
	if node := s.Nodes[id]; node != nil && !node.Directory && node.UserID == userID && node.UploadKey != "" {
		return &UploadStatus{ID: id, Name: node.Name, Size: node.Size, Offset: node.Size, ChunkSize: ChunkSize, Node: node}, nil
	}
	return nil, os.ErrNotExist
}

func reserveExtension() error {
	layoutLock.Lock()
	defer layoutLock.Unlock()
	lfsRoot := ""
	if setting.LFS.Storage != nil {
		lfsRoot = setting.LFS.Storage.Path
	}
	_, err := repolayout.ReserveExtension(setting.AppDataPath, setting.RepoRootPath, lfsRoot, "software-center")
	return err
}

func BeginUpload(ctx context.Context, orgID, userID int64, parent, name, clientKey string, size int64) (*UploadStatus, error) {
	if size < 0 || size > MaxQuota || !hexDigest(clientKey) {
		return nil, ErrInvalid
	}
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return nil, err
	}
	prune(orgID, s)
	count, total := 0, 0
	for id, r := range s.Uploads {
		if r.Resumable {
			total++
			if r.UserID == userID {
				count++
			}
		}
		if r.Resumable && r.UserID == userID && r.Parent == parent && r.Name == name && r.Size == size && r.ClientKey == clientKey {
			return uploadStatus(id, r), nil
		}
	}
	for _, n := range s.Nodes {
		if !n.Directory && n.Parent == parent && n.Name == name && n.Size == size && n.UploadKey == clientKey && n.UserID == userID {
			return &UploadStatus{ID: n.ID, Name: n.Name, Size: n.Size, Offset: n.Size, ChunkSize: ChunkSize, Node: n}, nil
		}
	}
	release, err := reserveDisk(size)
	if err != nil {
		return nil, err
	}
	release()
	if count >= 32 || total >= 128 {
		return nil, ErrBusy
	}
	if size > s.Quota-s.Used()-s.Reserved() || len(s.Nodes)+len(s.Uploads) >= MaxEntries {
		return nil, ErrQuota
	}
	if err = s.checkName(parent, name, ""); err != nil {
		return nil, err
	}
	if err = reserveExtension(); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(root(orgID), "blobs"), 0o700); err != nil {
		return nil, err
	}
	id := newID()
	part := filepath.Join(root(orgID), id+".part")
	f, err := os.OpenFile(part, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}
	state, _ := sha256.New().(encoding.BinaryMarshaler).MarshalBinary()
	r := Reservation{Parent: parent, Name: name, Size: size, Started: time.Now().Unix(), UserID: userID, Resumable: true, ClientKey: clientKey, HashState: state}
	s.Uploads[id] = r
	if err = save(ctx, orgID, s); err != nil {
		_ = os.Remove(part)
		return nil, err
	}
	return uploadStatus(id, r), nil
}

type runningCRC struct{ sum uint32 }

func (c *runningCRC) Write(p []byte) (int, error) {
	c.sum = crc32.Update(c.sum, crc32.IEEETable, p)
	return len(p), nil
}

type contextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(p)
}

func AppendUpload(ctx context.Context, orgID, userID int64, id string, offset, size int64, digest string, body io.Reader) (*UploadStatus, error) {
	if !validID(id) || size <= 0 || size > ChunkSize || !hexDigest(digest) {
		return nil, ErrInvalid
	}
	m := sessionLock(id)
	m.Lock()
	defer m.Unlock()
	guard := lock(orgID)
	guard.Lock()
	s, err := Load(ctx, orgID)
	if err != nil {
		guard.Unlock()
		return nil, err
	}
	r, ok := s.Uploads[id]
	if !ok || !r.Resumable || r.UserID != userID {
		guard.Unlock()
		return nil, os.ErrNotExist
	}
	if r.Offset != offset {
		guard.Unlock()
		return nil, ErrConflict
	}
	if size != min(ChunkSize, r.Size-offset) {
		guard.Unlock()
		return nil, ErrInvalid
	}
	guard.Unlock()
	select {
	case uploadSlots <- struct{}{}:
		defer func() { <-uploadSlots }()
	default:
		return nil, ErrBusy
	}
	release, err := reserveDisk(size)
	if err != nil {
		return nil, err
	}
	defer release()
	f, err := os.OpenFile(filepath.Join(root(orgID), id+".part"), os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < offset {
		return nil, ErrInvalid
	}
	if err = f.Truncate(offset); err != nil {
		return nil, err
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	whole := sha256.New()
	if err = whole.(encoding.BinaryUnmarshaler).UnmarshalBinary(r.HashState); err != nil {
		return nil, ErrInvalid
	}
	chunk := sha256.New()
	checksum := &runningCRC{sum: r.CRC32}
	n, err := io.CopyBuffer(io.MultiWriter(f, whole, chunk, checksum), io.LimitReader(contextReader{ctx, body}, size+1), make([]byte, 128*1024))
	if err != nil || n != size || hex.EncodeToString(chunk.Sum(nil)) != digest {
		_ = f.Truncate(offset)
		if err != nil {
			return nil, err
		}
		return nil, ErrInvalid
	}
	if err = f.Sync(); err != nil {
		_ = f.Truncate(offset)
		return nil, err
	}
	previousChain := r.Chain
	r.Offset += size
	r.CRC32 = checksum.sum
	r.Started = time.Now().Unix()
	r.HashState, _ = whole.(encoding.BinaryMarshaler).MarshalBinary()
	chain := sha256.Sum256([]byte(previousChain + ":" + digest + ":" + strconv.FormatInt(r.Offset, 10)))
	r.Chain = hex.EncodeToString(chain[:])
	guard.Lock()
	defer guard.Unlock()
	s, err = Load(ctx, orgID)
	if err != nil {
		_ = f.Truncate(offset)
		return nil, err
	}
	current, exists := s.Uploads[id]
	if !exists || !current.Resumable || current.UserID != userID || current.Offset != offset || current.Chain != previousChain || current.Size != r.Size {
		_ = f.Truncate(offset)
		return nil, ErrConflict
	}
	s.Uploads[id] = r
	if err = save(ctx, orgID, s); err != nil {
		_ = f.Truncate(offset)
		return nil, err
	}
	return uploadStatus(id, r), nil
}

func CompleteUpload(ctx context.Context, orgID, userID int64, id string) (*Node, error) {
	if !validID(id) {
		return nil, ErrInvalid
	}
	m := sessionLock(id)
	m.Lock()
	defer m.Unlock()
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if n := s.Nodes[id]; n != nil && n.UserID == userID && n.UploadKey != "" {
		return n, nil
	}
	r, ok := s.Uploads[id]
	if !ok || !r.Resumable || r.UserID != userID {
		return nil, os.ErrNotExist
	}
	if r.Offset != r.Size || !s.folder(r.Parent) {
		return nil, ErrConflict
	}
	part := filepath.Join(root(orgID), id+".part")
	info, err := os.Stat(part)
	if err != nil {
		return nil, err
	}
	if info.Size() != r.Size {
		return nil, ErrConflict
	}
	h := sha256.New()
	if err = h.(encoding.BinaryUnmarshaler).UnmarshalBinary(r.HashState); err != nil {
		return nil, ErrInvalid
	}
	digest := hex.EncodeToString(h.Sum(nil))
	// The cleanup intent is durable before linking, including a crash before metadata finalization.
	s.Garbage[digest] = time.Now().Unix()
	if err = save(ctx, orgID, s); err != nil {
		return nil, err
	}
	blob := filepath.Join(root(orgID), "blobs", digest)
	if info, statErr := os.Stat(blob); statErr == nil {
		if info.Size() != r.Size {
			return nil, ErrInvalid
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	} else if err = os.Link(part, blob); err != nil {
		return nil, err
	}
	crc := r.CRC32
	n := &Node{ID: id, Parent: r.Parent, Name: r.Name, Size: r.Size, Digest: digest, Modified: time.Now().Unix(), UserID: userID, CRC32: &crc, UploadKey: r.ClientKey}
	s.Nodes[id] = n
	delete(s.Uploads, id)
	delete(s.Garbage, digest)
	s.Garbage["part:"+id] = time.Now().Unix()
	s.Generation++
	if err = save(ctx, orgID, s); err != nil {
		return nil, err
	}
	reclaim(orgID, s)
	_ = save(ctx, orgID, s)
	return n, nil
}

func CancelUpload(ctx context.Context, orgID, userID int64, id string) error {
	if !validID(id) {
		return ErrInvalid
	}
	m := sessionLock(id)
	m.Lock()
	defer m.Unlock()
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return err
	}
	r, ok := s.Uploads[id]
	if !ok || !r.Resumable || r.UserID != userID {
		return os.ErrNotExist
	}
	delete(s.Uploads, id)
	s.Garbage["part:"+id] = time.Now().Unix()
	if err = save(ctx, orgID, s); err != nil {
		return err
	}
	reclaim(orgID, s)
	_ = save(ctx, orgID, s)
	return nil
}
