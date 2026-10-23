// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package softwarecenter implements a single-writer organization file share outside Git/LFS.
package softwarecenter

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/models/system"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/repolayout"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/storage/awareness"
	"golang.org/x/text/unicode/norm"
)

const (
	DefaultQuota int64 = 100 * 1024 * 1024 * 1024
	MaxQuota     int64 = 10 * 1024 * 1024 * 1024 * 1024
	MaxEntries         = 20000
)

var (
	ErrInvalid      = errors.New("invalid file or folder")
	ErrConflict     = errors.New("name already exists or folder changed")
	ErrQuota        = errors.New("organization storage quota exceeded")
	ErrBusy         = errors.New("upload capacity busy")
	ErrDiskCapacity = errors.New("insufficient unreserved disk capacity")
	locks           sync.Map
	layoutLock      sync.Mutex
	uploadSlots     = make(chan struct{}, 8)
	diskLock        sync.Mutex
	diskReserved    int64
	snapshotLock    sync.Mutex
	snapshots       = make(map[int64]stateSnapshot)
)

type stateSnapshot struct {
	state   *State
	expires time.Time
}

func reserveDisk(size int64) (func(), error) {
	diskLock.Lock()
	defer diskLock.Unlock()
	service, err := awareness.NewLiveService(awareness.LiveOptions{VolumePath: setting.AppDataPath, ReserveBytes: 2 * 1024 * 1024 * 1024})
	if err != nil {
		return nil, err
	}
	if !service.Admit(size + diskReserved).Allowed {
		return nil, ErrDiskCapacity
	}
	diskReserved += size
	return func() { diskLock.Lock(); diskReserved -= size; diskLock.Unlock() }, nil
}

type Node struct {
	ID        string  `json:"id"`
	Parent    string  `json:"parent"`
	Name      string  `json:"name"`
	Directory bool    `json:"directory"`
	Size      int64   `json:"size"`
	Digest    string  `json:"digest,omitempty"`
	Modified  int64   `json:"modified"`
	UserID    int64   `json:"user_id"`
	CRC32     *uint32 `json:"crc32,omitempty"`
	UploadKey string  `json:"upload_key,omitempty"`
}

// SoftwareNodeInfo is the listing projection; upload keys and owner IDs never leave the server.
type SoftwareNodeInfo struct {
	ID        string `json:"id"`
	Parent    string `json:"parent"`
	Name      string `json:"name"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest,omitempty"`
	Modified  int64  `json:"modified"`
}

func publicNode(node *Node) *SoftwareNodeInfo {
	return &SoftwareNodeInfo{ID: node.ID, Parent: node.Parent, Name: node.Name, Directory: node.Directory, Size: node.Size, Digest: node.Digest, Modified: node.Modified}
}

type Reservation struct {
	Parent    string `json:"parent"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Started   int64  `json:"started"`
	UserID    int64  `json:"user_id,omitempty"`
	Resumable bool   `json:"resumable,omitempty"`
	Offset    int64  `json:"offset,omitempty"`
	ClientKey string `json:"client_key,omitempty"`
	Chain     string `json:"chain,omitempty"`
	HashState []byte `json:"hash_state,omitempty"`
	CRC32     uint32 `json:"crc32,omitempty"`
}
type State struct {
	Version    int                     `json:"version"`
	Generation int64                   `json:"generation,omitempty"`
	Quota      int64                   `json:"quota"`
	Nodes      map[string]*Node        `json:"nodes"`
	Uploads    map[string]Reservation  `json:"uploads"`
	Garbage    map[string]int64        `json:"garbage,omitempty"`
	Downloads  map[string]*DownloadJob `json:"downloads,omitempty"`
}

func lock(orgID int64) *sync.Mutex {
	value, _ := locks.LoadOrStore(orgID, &sync.Mutex{})
	return value.(*sync.Mutex)
}
func key(orgID int64) string { return fmt.Sprintf("software-center-v1-%d", orgID) }
func root(orgID int64) string {
	return filepath.Join(repolayout.ExtensionsPath(setting.AppDataPath), "software-center", strconv.FormatInt(orgID, 10))
}

func newID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes[:])
}

func Load(ctx context.Context, orgID int64) (*State, error) {
	raw, err := system.GetAppStateContent(ctx, key(orgID))
	if err != nil {
		return nil, err
	}
	state := &State{Version: 1, Quota: DefaultQuota, Nodes: map[string]*Node{}, Uploads: map[string]Reservation{}}
	if raw != "" {
		if err = json.Unmarshal([]byte(raw), state); err != nil {
			return nil, err
		}
	}
	if state.Version != 1 || state.Quota <= 0 || state.Nodes == nil || state.Uploads == nil {
		return nil, ErrInvalid
	}
	if state.Garbage == nil {
		state.Garbage = map[string]int64{}
	}
	if state.Downloads == nil {
		state.Downloads = map[string]*DownloadJob{}
	}
	return state, nil
}

func save(ctx context.Context, orgID int64, state *State) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err = system.SaveAppStateContent(ctx, key(orgID), string(raw)); err != nil {
		return err
	}
	snapshotLock.Lock()
	delete(snapshots, orgID)
	snapshotLock.Unlock()
	return nil
}

// Snapshot is immutable and short-lived; membership is checked separately on every HTTP request.
func Snapshot(ctx context.Context, orgID int64) (*State, error) {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	snapshotLock.Lock()
	cached, ok := snapshots[orgID]
	snapshotLock.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return cached.state, nil
	}
	state, err := Load(ctx, orgID)
	if err != nil {
		return nil, err
	}
	snapshotLock.Lock()
	defer snapshotLock.Unlock()
	if len(snapshots) >= 8 {
		var oldest int64
		var expiry time.Time
		for id, item := range snapshots {
			if expiry.IsZero() || item.expires.Before(expiry) {
				oldest = id
				expiry = item.expires
			}
		}
		delete(snapshots, oldest)
	}
	snapshots[orgID] = stateSnapshot{state: state, expires: time.Now().Add(15 * time.Second)}
	return state, nil
}

func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func validName(name string) bool {
	if name == "" || len(name) > 255 || name == "." || name == ".." || strings.TrimSpace(name) != name || strings.HasSuffix(name, ".") || strings.ContainsAny(name, "/\\\x00\r\n\t") {
		return false
	}
	for _, char := range name {
		if char < 32 || char == 127 {
			return false
		}
	}
	return norm.NFC.IsNormalString(name)
}

func (s *State) Used() int64 {
	var used int64
	for _, node := range s.Nodes {
		used += node.Size
	}
	return used
}

func (s *State) Reserved() int64 {
	var used int64
	for _, r := range s.Uploads {
		used += r.Size
	}
	return used
}

func (s *State) folder(id string) bool {
	return id == "" || (s.Nodes[id] != nil && s.Nodes[id].Directory)
}

func (s *State) checkName(parent, name, except string) error {
	if !s.folder(parent) || !validName(name) {
		return ErrInvalid
	}
	depth := 0
	for id := parent; id != ""; id = s.Nodes[id].Parent {
		depth++
		if depth >= 32 {
			return ErrInvalid
		}
	}
	for id, node := range s.Nodes {
		if id != except && node.Parent == parent && strings.EqualFold(node.Name, name) {
			return ErrConflict
		}
	}
	for _, r := range s.Uploads {
		if r.Parent == parent && strings.EqualFold(r.Name, name) {
			return ErrConflict
		}
	}
	return nil
}

func prune(orgID int64, s *State) {
	reclaim(orgID, s)
	for id, r := range s.Uploads {
		retention := 24 * time.Hour
		if r.Resumable {
			retention = 7 * 24 * time.Hour
		}
		if r.Started < time.Now().Add(-retention).Unix() {
			delete(s.Uploads, id)
			if validID(id) {
				s.Garbage["part:"+id] = time.Now().Unix()
			}
		}
	}
	for id, job := range s.Downloads {
		if job.Expires < time.Now().Unix() {
			delete(s.Downloads, id)
			deleteArchivePlan(id)
		}
	}
	reclaim(orgID, s)
}

func reclaim(orgID int64, s *State) {
	referenced := make(map[string]bool, len(s.Nodes))
	for _, node := range s.Nodes {
		referenced[node.Digest] = true
	}
	for digest := range s.Garbage {
		if id, ok := strings.CutPrefix(digest, "part:"); ok {
			if validID(id) {
				if err := os.Remove(filepath.Join(root(orgID), id+".part")); err == nil || errors.Is(err, os.ErrNotExist) {
					delete(s.Garbage, digest)
				}
			}
			continue
		}
		if referenced[digest] {
			delete(s.Garbage, digest)
			continue
		}
		if len(digest) != 64 {
			continue
		}
		if _, err := hex.DecodeString(digest); err != nil {
			continue
		}
		if err := os.Remove(filepath.Join(root(orgID), "blobs", digest)); err == nil || errors.Is(err, os.ErrNotExist) {
			delete(s.Garbage, digest)
		} else {
			log.Warn("Software center cleanup pending for organization %d: %v", orgID, err)
		}
	}
}

func CreateFolder(ctx context.Context, orgID, userID int64, parent, name string) (*Node, error) {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return nil, err
	}
	prune(orgID, s)
	if len(s.Nodes)+len(s.Uploads) >= MaxEntries {
		return nil, ErrQuota
	}
	if err = s.checkName(parent, name, ""); err != nil {
		return nil, err
	}
	node := &Node{ID: newID(), Parent: parent, Name: name, Directory: true, Modified: time.Now().Unix(), UserID: userID}
	s.Nodes[node.ID] = node
	s.Generation++
	return node, save(ctx, orgID, s)
}

func SetQuota(ctx context.Context, orgID, quota int64) error {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return err
	}
	prune(orgID, s)
	if quota < 1 || quota > MaxQuota || quota < s.Used()+s.Reserved() {
		return ErrQuota
	}
	s.Quota = quota
	return save(ctx, orgID, s)
}

func Rename(ctx context.Context, orgID int64, id, name string) error {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return err
	}
	node := s.Nodes[id]
	if node == nil {
		return ErrInvalid
	}
	if err = s.checkName(node.Parent, name, id); err != nil {
		return err
	}
	node.Name = name
	node.Modified = time.Now().Unix()
	s.Generation++
	return save(ctx, orgID, s)
}

func childIndex(s *State) map[string][]string {
	index := make(map[string][]string, len(s.Nodes))
	for id, node := range s.Nodes {
		index[node.Parent] = append(index[node.Parent], id)
	}
	return index
}

func descendantsFrom(index map[string][]string, id string) map[string]bool {
	result := map[string]bool{}
	stack := []string{id}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if result[current] {
			continue
		}
		result[current] = true
		stack = append(stack, index[current]...)
	}
	return result
}

func descendants(s *State, id string) map[string]bool {
	return descendantsFrom(childIndex(s), id)
}

func Delete(ctx context.Context, orgID int64, id string) error {
	return DeleteMany(ctx, orgID, []string{id})
}

func Upload(ctx context.Context, orgID, userID int64, parent, name string, size int64, body io.Reader) (*Node, error) {
	if size < 0 || size > MaxQuota {
		return nil, ErrQuota
	}
	select {
	case uploadSlots <- struct{}{}:
		defer func() { <-uploadSlots }()
	default:
		return nil, ErrBusy
	}
	releaseDisk, err := reserveDisk(size)
	if err != nil {
		return nil, err
	}
	defer releaseDisk()
	guard := lock(orgID)
	id := newID()
	guard.Lock()
	s, err := Load(ctx, orgID)
	if err == nil {
		prune(orgID, s)
		if size < 0 || size > s.Quota-s.Used()-s.Reserved() || len(s.Nodes)+len(s.Uploads) >= MaxEntries {
			err = ErrQuota
		}
	}
	if err == nil {
		err = s.checkName(parent, name, "")
	}
	if err == nil {
		s.Uploads[id] = Reservation{Parent: parent, Name: name, Size: size, Started: time.Now().Unix()}
		err = save(ctx, orgID, s)
	}
	guard.Unlock()
	if err != nil {
		return nil, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		guard.Lock()
		defer guard.Unlock()
		current, loadErr := Load(cleanup, orgID)
		if loadErr == nil {
			if _, exists := current.Uploads[id]; exists {
				delete(current.Uploads, id)
				_ = save(cleanup, orgID, current)
			}
		}
		_ = os.Remove(filepath.Join(root(orgID), id+".part"))
	}()
	layoutLock.Lock()
	lfsRoot := ""
	if setting.LFS.Storage != nil {
		lfsRoot = setting.LFS.Storage.Path
	}
	_, err = repolayout.ReserveExtension(setting.AppDataPath, setting.RepoRootPath, lfsRoot, "software-center")
	layoutLock.Unlock()
	if err != nil {
		return nil, err
	}
	directory := root(orgID)
	if err = os.MkdirAll(filepath.Join(directory, "blobs"), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, id+".part"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	digest := sha256.New()
	checksum := crc32.NewIEEE()
	count, copyErr := io.CopyBuffer(io.MultiWriter(file, digest, checksum), io.LimitReader(body, size+1), make([]byte, 128*1024))
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if syncErr != nil {
		return nil, syncErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if count != size {
		return nil, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	hash := hex.EncodeToString(digest.Sum(nil))
	guard.Lock()
	defer guard.Unlock()
	s, err = Load(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if _, exists := s.Uploads[id]; !exists || !s.folder(parent) {
		return nil, ErrConflict
	}
	blob := filepath.Join(directory, "blobs", hash)
	if info, statErr := os.Stat(blob); statErr == nil {
		if info.Size() != size {
			return nil, ErrInvalid
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	} else if err = os.Link(filepath.Join(directory, id+".part"), blob); err != nil {
		return nil, err
	}
	node := &Node{ID: id, Parent: parent, Name: name, Size: size, Digest: hash, Modified: time.Now().Unix(), UserID: userID}
	crc := checksum.Sum32()
	node.CRC32 = &crc
	delete(s.Uploads, id)
	delete(s.Garbage, hash)
	s.Nodes[id] = node
	s.Generation++
	if err = save(ctx, orgID, s); err != nil {
		referenced := false
		for nodeID, other := range s.Nodes {
			if nodeID != id && other.Digest == hash {
				referenced = true
				break
			}
		}
		if !referenced {
			_ = os.Remove(blob)
		}
		return nil, err
	}
	return node, nil
}

func Open(ctx context.Context, orgID int64, id string) (*Node, *os.File, error) {
	s, err := Snapshot(ctx, orgID)
	if err != nil {
		return nil, nil, err
	}
	node := s.Nodes[id]
	if node == nil || node.Directory || len(node.Digest) != 64 {
		return nil, nil, os.ErrNotExist
	}
	if _, err = hex.DecodeString(node.Digest); err != nil {
		return nil, nil, os.ErrNotExist
	}
	file, err := os.Open(filepath.Join(root(orgID), "blobs", node.Digest))
	return node, file, err
}
