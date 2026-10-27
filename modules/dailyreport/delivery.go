// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/neko233-com/gitsss/modules/json"
)

// FileDeliveryStore persists successful deliveries outside the source tree.
type FileDeliveryStore struct{ Path string }

func (s FileDeliveryStore) read() (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool)
	info, err := os.Lstat(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return nil, errors.New("invalid delivery state")
	}
	data, err := os.ReadFile(s.Path)
	if err != nil || json.Unmarshal(data, &result) != nil || result == nil {
		return nil, errors.New("invalid delivery state")
	}
	return result, nil
}

// Delivered checks whether the card has already been acknowledged.
func (s FileDeliveryStore) Delivered(date, account string) (bool, error) {
	data, err := s.read()
	if err != nil {
		return false, err
	}
	return data[date][account], nil
}

// MarkDelivered atomically records an acknowledged card.
func (s FileDeliveryStore) MarkDelivered(date, account string) error {
	if date == "" || account == "" {
		return errors.New("invalid delivery identity")
	}
	data, err := s.read()
	if err != nil {
		return err
	}
	if data[date] == nil {
		data[date] = make(map[string]bool)
	}
	data[date][account] = true
	encoded, err := json.Marshal(data)
	if err != nil {
		return errors.New("cannot encode delivery state")
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.New("cannot create delivery state directory")
	}
	f, err := os.CreateTemp(dir, ".delivery-*")
	if err != nil {
		return errors.New("cannot create delivery state")
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return errors.New("cannot protect delivery state")
	}
	if _, err := f.Write(encoded); err != nil {
		f.Close()
		return errors.New("cannot write delivery state")
	}
	if err := f.Close(); err != nil {
		return errors.New("cannot close delivery state")
	}
	if err := os.Rename(f.Name(), s.Path); err != nil {
		return errors.New("cannot replace delivery state")
	}
	return nil
}
