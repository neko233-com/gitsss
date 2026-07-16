// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ver

// go build -ldflags "-X github.com/neko233-com/gitsss/runner/internal/pkg/ver.version=1.2.3"
var version = "dev"

func Version() string {
	return version
}
