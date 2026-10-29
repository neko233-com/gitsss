// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/neko233-com/gitsss/modules/json"
)

// Config is private server state; it must not be returned to ordinary users.
type Config struct {
	Schedule          Schedule          `json:"schedule"`
	BaseURL           string            `json:"base_url"`
	Model             string            `json:"model"`
	APIKey            string            `json:"api_key"`
	Webhook           string            `json:"webhook"`
	OmitStore         bool              `json:"omit_store"`
	Accounts          map[string]string `json:"accounts"`
	RoleFilterEnabled bool              `json:"role_filter_enabled,omitempty"`
	RoleLabels        []string          `json:"role_labels,omitempty"`
}

// Validate rejects ambiguous classifications and invalid calendar entries.
func (c Config) Validate() error {
	if _, err := time.Parse("15:04", c.Schedule.Time); err != nil || len(c.Schedule.Time) != 5 {
		return errors.New("invalid daily report time")
	}
	years := make(map[int]bool, len(c.Schedule.CalendarYears))
	for _, year := range c.Schedule.CalendarYears {
		if year < 1970 || year > 9999 || years[year] {
			return errors.New("invalid holiday calendar year")
		}
		years[year] = true
	}
	days := make(map[string]bool)
	for _, dates := range [][]string{c.Schedule.Holidays, c.Schedule.Workdays} {
		for _, date := range dates {
			parsed, err := time.Parse("2006-01-02", date)
			if err != nil || parsed.Format("2006-01-02") != date || days[date] || !years[parsed.Year()] {
				return errors.New("invalid holiday calendar date")
			}
			days[date] = true
		}
	}
	for name, kind := range c.Accounts {
		if strings.TrimSpace(name) != name || name == "" || (kind != "person" && kind != "service") {
			return errors.New("invalid account classification")
		}
	}
	if c.RoleFilterEnabled && len(c.RoleLabels) == 0 {
		return errors.New("daily report role filter requires at least one label")
	}
	if len(c.RoleLabels) > 1000 {
		return errors.New("too many daily report role labels")
	}
	seenLabels := make(map[string]bool)
	for _, label := range c.RoleLabels {
		key := strings.ToLower(label)
		if label == "" || strings.TrimSpace(label) != label || utf8.RuneCountInString(label) > 32 || strings.IndexFunc(label, unicode.IsControl) >= 0 || seenLabels[key] {
			return errors.New("invalid daily report role label")
		}
		seenLabels[key] = true
	}
	if c.Schedule.Enabled && (c.BaseURL == "" || c.Model == "" || c.APIKey == "" || c.Webhook == "") {
		return errors.New("enabled daily report requires provider and webhook configuration")
	}
	if c.BaseURL != "" {
		u, err := url.Parse(c.BaseURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("invalid responses endpoint")
		}
	}
	if c.Webhook != "" {
		u, err := url.Parse(c.Webhook)
		if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "open.feishu.cn") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/open-apis/bot/v2/hook/") || strings.TrimPrefix(u.Path, "/open-apis/bot/v2/hook/") == "" {
			return errors.New("invalid Feishu webhook")
		}
	}
	return nil
}

// LoadConfig reads an explicitly selected private configuration file.
func LoadConfig(path string) (Config, error) {
	var c Config
	info, err := os.Lstat(path)
	if err != nil {
		return c, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return c, errors.New("invalid daily report configuration file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, errors.New("cannot read daily report configuration")
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, errors.New("invalid daily report configuration")
	}
	return c, c.Validate()
}

// SaveConfig writes private configuration atomically with owner-only permissions.
func SaveConfig(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.New("cannot prepare daily report configuration directory")
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("invalid daily report configuration file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("cannot inspect daily report configuration")
	}
	data, err := json.Marshal(c)
	if err != nil {
		return errors.New("cannot encode daily report configuration")
	}
	f, err := os.CreateTemp(dir, ".dailyreport-*")
	if err != nil {
		return errors.New("cannot create daily report configuration")
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return errors.New("cannot protect daily report configuration")
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return errors.New("cannot write daily report configuration")
	}
	if err := f.Close(); err != nil {
		return errors.New("cannot close daily report configuration")
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return errors.New("cannot replace daily report configuration")
	}
	return nil
}
