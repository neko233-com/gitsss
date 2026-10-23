// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "github.com/neko233-com/gitsss/modules/orgshare"

// OrgShareSetting is the organization file-share capacity. PATH is separate from Git and LFS.
// MaxBytes is the default per-organization cap. A site admin can override it per organization.
type OrgShareSetting struct {
	Enabled      bool
	MaxBytes     int64
	MaxFileBytes int64
	ExpireDays   int
	ReserveBytes int64
	Path         string
}

// OrgShare is loaded from [org_share].
var OrgShare OrgShareSetting

func loadOrgShareFrom(rootCfg ConfigProvider) {
	OrgShare = OrgShareSetting{
		Enabled:      true,
		MaxBytes:     orgshare.DefaultMaxBytes,
		MaxFileBytes: orgshare.DefaultMaxFileBytes,
		ExpireDays:   orgshare.DefaultExpireDays,
		ReserveBytes: orgshare.DefaultReserveBytes,
	}
	sec, err := rootCfg.GetSection("org_share")
	if err != nil || sec == nil {
		return
	}
	OrgShare.Enabled = sec.Key("ENABLED").MustBool(true)
	OrgShare.MaxBytes = sec.Key("MAX_BYTES").MustInt64(orgshare.DefaultMaxBytes)
	OrgShare.MaxFileBytes = sec.Key("MAX_FILE_BYTES").MustInt64(orgshare.DefaultMaxFileBytes)
	OrgShare.ExpireDays = sec.Key("EXPIRE_DAYS").MustInt(orgshare.DefaultExpireDays)
	OrgShare.ReserveBytes = sec.Key("RESERVE_BYTES").MustInt64(orgshare.DefaultReserveBytes)
	OrgShare.Path = sec.Key("PATH").MustString("")
}
