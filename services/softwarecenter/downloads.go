// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package softwarecenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/modules/json"
)

type DownloadJob struct {
	ID      string   `json:"id"`
	UserID  int64    `json:"user_id"`
	IDs     []string `json:"ids"`
	Name    string   `json:"name"`
	ETag    string   `json:"etag"`
	Size    int64    `json:"size"`
	Created int64    `json:"created"`
	Expires int64    `json:"expires"`
	Single  bool     `json:"single"`
	State   string   `json:"state,omitempty"`
	Bytes   int64    `json:"bytes,omitempty"`
	Speed   float64  `json:"speed,omitempty"`
}

type ArchiveEntry struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	CRC32     uint32 `json:"crc32"`
	Modified  int64  `json:"modified"`
}

var archiveWorkers = make(chan struct{}, 2)

func archiveEntries(s *State, ids []string) ([]ArchiveEntry, error) {
	top, err := selection(s, ids)
	if err != nil {
		return nil, err
	}
	children := map[string][]*Node{}
	for _, n := range s.Nodes {
		children[n.Parent] = append(children[n.Parent], n)
	}
	entries := []ArchiveEntry{}
	names := map[string]bool{}
	nameBytes := 0
	var appendNode func(*Node, string) error
	appendNode = func(n *Node, path string) error {
		if n.Directory {
			path += "/"
		}
		if names[strings.ToLower(path)] {
			return ErrConflict
		}
		names[strings.ToLower(path)] = true
		nameBytes += len(path)
		if len(path) > 65535 || nameBytes > 4*1024*1024 || len(entries) >= MaxEntries {
			return ErrQuota
		}
		e := ArchiveEntry{ID: n.ID, Path: path, Directory: n.Directory, Digest: n.Digest, Size: n.Size, Modified: n.Modified}
		if n.CRC32 != nil {
			e.CRC32 = *n.CRC32
		}
		entries = append(entries, e)
		if n.Directory {
			for _, child := range children[n.ID] {
				if err := appendNode(child, path+child.Name); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, id := range top {
		if err = appendNode(s.Nodes[id], s.Nodes[id].Name); err != nil {
			return nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func archiveTag(entries []ArchiveEntry) string {
	raw, _ := json.Marshal(entries)
	h := sha256.Sum256(raw)
	return `"zip-v1-` + hex.EncodeToString(h[:]) + `"`
}

func CreateDownload(ctx context.Context, orgID, userID int64, ids []string) (*DownloadJob, error) {
	select {
	case archiveWorkers <- struct{}{}:
		defer func() { <-archiveWorkers }()
	default:
		return nil, ErrBusy
	}
	s, err := Snapshot(ctx, orgID)
	if err != nil {
		return nil, err
	}
	entries, err := archiveEntries(s, ids)
	if err != nil {
		return nil, err
	}
	checksums := map[string]uint32{}
	for _, entry := range entries {
		if entry.Directory || s.Nodes[entry.ID].CRC32 != nil || (len(entries) == 1 && !entry.Directory) {
			continue
		}
		if _, known := checksums[entry.Digest]; known {
			continue
		}
		f, err := os.Open(filepath.Join(root(orgID), "blobs", entry.Digest))
		if err != nil {
			return nil, err
		}
		h := crc32.NewIEEE()
		n, copyErr := io.CopyBuffer(h, contextReader{ctx, f}, make([]byte, 128*1024))
		_ = f.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if n != entry.Size {
			return nil, ErrConflict
		}
		checksums[entry.Digest] = h.Sum32()
	}
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	current, err := Load(ctx, orgID)
	if err != nil {
		return nil, err
	}
	prune(orgID, current)
	// Detect changes during lazy legacy checksum preparation before caching or issuing a ticket.
	before, err := archiveEntries(current, ids)
	if err != nil {
		return nil, err
	}
	if len(before) != len(entries) {
		return nil, ErrConflict
	}
	for i := range before {
		before[i].CRC32 = entries[i].CRC32
	}
	if archiveTag(before) != archiveTag(entries) {
		return nil, ErrConflict
	}
	for _, entry := range before {
		if crc, ok := checksums[entry.Digest]; ok {
			value := crc
			current.Nodes[entry.ID].CRC32 = &value
		}
	}
	entries, err = archiveEntries(current, ids)
	if err != nil {
		return nil, err
	}
	count := 0
	for _, job := range current.Downloads {
		if job.UserID == userID {
			count++
		}
	}
	if len(current.Downloads) >= 128 || count >= 32 {
		return nil, ErrBusy
	}
	job := &DownloadJob{ID: newID(), UserID: userID, IDs: ids, Name: "software-center.zip", ETag: archiveTag(entries), Created: time.Now().Unix(), Expires: time.Now().Add(24 * time.Hour).Unix()}
	if len(entries) == 1 && !entries[0].Directory {
		job.Single = true
		job.Name = current.Nodes[entries[0].ID].Name
		job.Size = entries[0].Size
		job.ETag = `"sha256-` + entries[0].Digest + `"`
	} else {
		archive, err := newVirtualZIP(orgID, entries)
		if err != nil {
			return nil, err
		}
		job.Size = archive.Size()
		_ = archive.Close()
		if len(ids) == 1 && current.Nodes[ids[0]].Directory {
			job.Name = current.Nodes[ids[0]].Name + ".zip"
		}
	}
	current.Downloads[job.ID] = job
	return job, save(ctx, orgID, current)
}

func GetDownload(ctx context.Context, orgID, userID int64, id string) (*DownloadJob, error) {
	s, err := Snapshot(ctx, orgID)
	if err != nil {
		return nil, err
	}
	j := s.Downloads[id]
	if j == nil || j.UserID != userID || j.Expires < time.Now().Unix() {
		return nil, os.ErrNotExist
	}
	return j, nil
}

func Downloads(ctx context.Context, orgID, userID int64) ([]*DownloadJob, error) {
	s, err := Snapshot(ctx, orgID)
	if err != nil {
		return nil, err
	}
	jobs := []*DownloadJob{}
	for _, job := range s.Downloads {
		if job.UserID == userID && job.Expires >= time.Now().Unix() {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Created > jobs[j].Created })
	return jobs, nil
}

func OpenDownload(ctx context.Context, orgID, userID int64, id string) (*DownloadJob, io.ReadSeekCloser, error) {
	job, err := GetDownload(ctx, orgID, userID, id)
	if err != nil {
		return nil, nil, err
	}
	s, err := Snapshot(ctx, orgID)
	if err != nil {
		return nil, nil, err
	}
	entries, err := archiveEntries(s, job.IDs)
	if err != nil {
		return nil, nil, ErrConflict
	}
	if job.Single {
		if len(entries) != 1 || `"sha256-`+entries[0].Digest+`"` != job.ETag || entries[0].Path != job.Name {
			return nil, nil, ErrConflict
		}
		f, err := os.Open(filepath.Join(root(orgID), "blobs", entries[0].Digest))
		return job, f, err
	}
	if archiveTag(entries) != job.ETag {
		return nil, nil, ErrConflict
	}
	file, err := newVirtualZIP(orgID, entries)
	return job, file, err
}

func RemoveDownload(ctx context.Context, orgID, userID int64, id string) error {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return err
	}
	j := s.Downloads[id]
	if j == nil || j.UserID != userID {
		return os.ErrNotExist
	}
	delete(s.Downloads, id)
	transferRecords.Delete(id)
	return save(ctx, orgID, s)
}

type TransferProgress struct {
	Bytes   int64   `json:"bytes"`
	Total   int64   `json:"total"`
	State   string  `json:"state"`
	Updated int64   `json:"updated"`
	Speed   float64 `json:"speed"`
}
type transferRecord struct {
	mu       sync.Mutex
	progress TransferProgress
}

var transferRecords sync.Map

func DownloadProgress(job *DownloadJob) TransferProgress {
	id, total := job.ID, job.Size
	if value, ok := transferRecords.Load(id); ok {
		r := value.(*transferRecord)
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.progress
	}
	state := job.State
	if state == "" {
		state = "queued"
	}
	return TransferProgress{Total: total, Bytes: job.Bytes, State: state, Speed: job.Speed}
}

func StartDownloadProgress(id string, total, offset int64) func(int64, string) {
	transferRecords.Range(func(key, value any) bool {
		r := value.(*transferRecord)
		r.mu.Lock()
		expired := r.progress.Updated < time.Now().Add(-24*time.Hour).Unix()
		r.mu.Unlock()
		if expired {
			transferRecords.Delete(key)
		}
		return true
	})
	started := time.Now()
	sent := int64(0)
	r := &transferRecord{progress: TransferProgress{Total: total, Bytes: offset, State: "downloading", Updated: time.Now().Unix()}}
	transferRecords.Store(id, r)
	return func(n int64, state string) {
		r.mu.Lock()
		defer r.mu.Unlock()
		sent += n
		r.progress.Bytes = min(r.progress.Total, r.progress.Bytes+n)
		r.progress.Speed = float64(sent) / max(time.Since(started).Seconds(), 0.01)
		r.progress.Updated = time.Now().Unix()
		if state != "" {
			r.progress.State = state
		}
	}
}

// FinishDownload persists one receipt per request; progress writes never touch the database.
func FinishDownload(ctx context.Context, orgID, userID int64, id string) error {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return err
	}
	job := s.Downloads[id]
	if job == nil || job.UserID != userID {
		return nil
	}
	progress := DownloadProgress(job)
	job.State, job.Bytes, job.Speed = progress.State, progress.Bytes, progress.Speed
	return save(ctx, orgID, s)
}
