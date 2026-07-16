// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !bindata

package templates

import (
	"github.com/neko233-com/gitsss/modules/assetfs"
	"github.com/neko233-com/gitsss/modules/setting"
)

func BuiltinAssets() *assetfs.Layer {
	return assetfs.Local("builtin(static)", setting.StaticRootPath, "templates")
}
