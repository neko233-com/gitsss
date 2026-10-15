// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"errors"
	"strings"

	"github.com/neko233-com/gitsss/modules/json"
)

// Field is one custom column on a task board.
type Field struct {
	Key   string   `json:"key"`
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Items []string `json:"items,omitempty"`
}

// ParseFields reads the board's custom field definitions.
func ParseFields(raw string) []Field {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var fields []Field
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil
	}
	return fields
}

// NormalizeFields checks administrator input before it is stored.
func NormalizeFields(fields []Field) ([]Field, error) {
	if len(fields) > 32 {
		return nil, errors.New("a board can have at most 32 custom fields")
	}
	seen := map[string]struct{}{}
	out := make([]Field, 0, len(fields))
	for _, field := range fields {
		field.Key = strings.TrimSpace(field.Key)
		field.Name = strings.TrimSpace(field.Name)
		field.Type = strings.TrimSpace(field.Type)
		if field.Key == "" || field.Name == "" {
			return nil, errors.New("custom field key and name are required")
		}
		if _, ok := seen[field.Key]; ok {
			return nil, errors.New("custom field keys must be unique")
		}
		switch field.Type {
		case "text", "number", "user":
			field.Items = nil
		case "select":
			if len(field.Items) == 0 || len(field.Items) > 50 {
				return nil, errors.New("select fields need between 1 and 50 items")
			}
		default:
			return nil, errors.New("custom field type must be text, number, user, or select")
		}
		for _, r := range field.Key {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			default:
				return nil, errors.New("custom field key must be lowercase letters, digits, or underscore")
			}
		}
		seen[field.Key] = struct{}{}
		out = append(out, field)
	}
	return out, nil
}

// EncodeFields stores normalized fields as JSON.
func EncodeFields(fields []Field) (string, error) {
	normalized, err := NormalizeFields(fields)
	if err != nil {
		return "", err
	}
	if len(normalized) == 0 {
		return "", nil
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
