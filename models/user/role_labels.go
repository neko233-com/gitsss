// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const SettingsKeyRoleLabels = "profile.role_labels"

// ParseRoleLabels validates and normalizes a comma-separated list of profile labels.
func ParseRoleLabels(input string) ([]string, error) {
	labels := make([]string, 0)
	seen := make(map[string]bool)
	for _, part := range strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == '，' || r == '\n' }) {
		label := strings.TrimSpace(part)
		if label == "" {
			continue
		}
		if utf8.RuneCountInString(label) > 32 || strings.IndexFunc(label, unicode.IsControl) >= 0 {
			return nil, errors.New("invalid label")
		}
		key := strings.ToLower(label)
		if !seen[key] {
			labels = append(labels, label)
			seen[key] = true
		}
	}
	if len(labels) > 20 {
		return nil, errors.New("too many labels")
	}
	return labels, nil
}
