package garbage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidToken = errors.New("garbage scan token is invalid or expired")
	ErrUnsafePath   = errors.New("garbage candidate is outside its managed root")
)

type Root struct {
	Kind   string
	Path   string
	MaxAge time.Duration
}

type Candidate struct {
	Kind     string
	Path     string
	Bytes    int64
	Files    int64
	Modified time.Time
}

type Report struct {
	Token      string
	ScannedAt  time.Time
	ExpiresAt  time.Time
	Bytes      int64
	Files      int64
	Candidates []Candidate
}

type Service struct {
	mu     sync.Mutex
	roots  []Root
	now    func() time.Time
	latest Report
}

func NewService(roots []Root) (*Service, error) {
	normalized := make([]Root, 0, len(roots))
	for _, root := range roots {
		if strings.TrimSpace(root.Kind) == "" || strings.TrimSpace(root.Path) == "" || root.MaxAge <= 0 {
			return nil, errors.New("invalid garbage root")
		}
		absolute, err := filepath.Abs(root.Path)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, Root{Kind: root.Kind, Path: filepath.Clean(absolute), MaxAge: root.MaxAge})
	}
	return &Service{roots: normalized, now: time.Now}, nil
}

func (s *Service) Latest() Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneReport(s.latest)
}

func (s *Service) Scan() (Report, error) {
	now := s.now().UTC()
	report := Report{Token: newToken(), ScannedAt: now, ExpiresAt: now.Add(15 * time.Minute), Candidates: []Candidate{}}
	for _, root := range s.roots {
		entries, err := os.ReadDir(root.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Report{}, fmt.Errorf("scan %s: %w", root.Path, err)
		}
		cutoff := now.Add(-root.MaxAge)
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil || info.Mode()&os.ModeSymlink != 0 || info.ModTime().After(cutoff) {
				continue
			}
			path := filepath.Join(root.Path, entry.Name())
			bytes, files, err := measure(path)
			if err != nil {
				return Report{}, fmt.Errorf("measure %s: %w", path, err)
			}
			report.Candidates = append(report.Candidates, Candidate{
				Kind: root.Kind, Path: path, Bytes: bytes, Files: files, Modified: info.ModTime().UTC(),
			})
			report.Bytes += bytes
			report.Files += files
		}
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		if report.Candidates[i].Bytes == report.Candidates[j].Bytes {
			return report.Candidates[i].Path < report.Candidates[j].Path
		}
		return report.Candidates[i].Bytes > report.Candidates[j].Bytes
	})
	s.mu.Lock()
	s.latest = cloneReport(report)
	s.mu.Unlock()
	return report, nil
}

func (s *Service) Clean(token string) (int64, int64, error) {
	s.mu.Lock()
	report := cloneReport(s.latest)
	s.mu.Unlock()
	now := s.now().UTC()
	if token == "" || token != report.Token || report.ExpiresAt.Before(now) {
		return 0, 0, ErrInvalidToken
	}
	var removedBytes, removedFiles int64
	for _, candidate := range report.Candidates {
		root, ok := s.rootFor(candidate)
		if !ok || !within(root.Path, candidate.Path) {
			return removedBytes, removedFiles, ErrUnsafePath
		}
		info, err := os.Lstat(candidate.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return removedBytes, removedFiles, err
		}
		if info.Mode()&os.ModeSymlink != 0 || info.ModTime().After(now.Add(-root.MaxAge)) {
			continue
		}
		if err := os.RemoveAll(candidate.Path); err != nil {
			return removedBytes, removedFiles, err
		}
		removedBytes += candidate.Bytes
		removedFiles += candidate.Files
	}
	s.mu.Lock()
	s.latest = Report{}
	s.mu.Unlock()
	return removedBytes, removedFiles, nil
}

func (s *Service) rootFor(candidate Candidate) (Root, bool) {
	for _, root := range s.roots {
		if root.Kind == candidate.Kind && within(root.Path, candidate.Path) {
			return root, true
		}
	}
	return Root{}, false
}

func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && relative != "" && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func measure(path string) (int64, int64, error) {
	var bytes, files int64
	err := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		bytes += info.Size()
		files++
		return nil
	})
	if err == nil && files == 0 {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			bytes, files = info.Size(), 1
		}
	}
	return bytes, files, err
}

func newToken() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(value[:])
}

func cloneReport(report Report) Report {
	report.Candidates = append([]Candidate(nil), report.Candidates...)
	return report
}
