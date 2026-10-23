// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"errors"
	"fmt"
)

// AdmitInput is the quota and disk check performed before bytes are accepted.
type AdmitInput struct {
	Used      int64
	Incoming  int64
	MaxBytes  int64
	MaxFile   int64
	Available int64
	Reserve   int64
	DiskKnown bool
}

// LimitError is a refusal that happened before any new chunk was stored.
type LimitError struct {
	Code      string
	Used      int64
	Limit     int64
	Incoming  int64
	Available int64
	Reserve   int64
}

func (e *LimitError) Error() string {
	switch e.Code {
	case "file":
		return fmt.Sprintf("文件超过单文件上限：本次 %s，上限 %s", formatBytes(e.Incoming), formatBytes(e.Limit))
	case "quota":
		return fmt.Sprintf("组织共享配额不足：已用 %s，上限 %s，本次 %s", formatBytes(e.Used), formatBytes(e.Limit), formatBytes(e.Incoming))
	case "disk":
		return fmt.Sprintf("磁盘空间不足：本次需要 %s，当前可用 %s，安全余量 %s", formatBytes(e.Incoming), formatBytes(e.Available), formatBytes(e.Reserve))
	case "disk_unknown":
		return "无法读取共享目录所在磁盘的可用空间，已拒绝上传"
	default:
		return "共享空间不足"
	}
}

// Admit rejects an upload that would pass the organization cap or the disk reserve.
func Admit(input AdmitInput) error {
	if input.Incoming < 0 || input.Used < 0 {
		return errors.New("share size must not be negative")
	}
	if input.MaxFile > 0 && input.Incoming > input.MaxFile {
		return &LimitError{Code: "file", Limit: input.MaxFile, Incoming: input.Incoming}
	}
	if input.MaxBytes > 0 && input.Used+input.Incoming > input.MaxBytes {
		return &LimitError{Code: "quota", Used: input.Used, Limit: input.MaxBytes, Incoming: input.Incoming}
	}
	if input.Incoming == 0 {
		return nil
	}
	if !input.DiskKnown {
		return &LimitError{Code: "disk_unknown", Incoming: input.Incoming}
	}
	if input.Reserve < 0 || input.Available < 0 {
		return errors.New("disk space must not be negative")
	}
	if input.Available-input.Reserve < input.Incoming {
		return &LimitError{
			Code: "disk", Incoming: input.Incoming, Available: input.Available, Reserve: input.Reserve,
		}
	}
	return nil
}

func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	suffix := []string{"KB", "MB", "GB", "TB"}
	for i, name := range suffix {
		value /= unit
		if value < unit || i == len(suffix)-1 {
			return fmt.Sprintf("%.1f %s", value, name)
		}
	}
	return fmt.Sprintf("%d B", n)
}
