// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package client

import (
	"github.com/neko233-com/gitsss/packages/actions-proto-go/ping/v1/pingv1connect"
	"github.com/neko233-com/gitsss/packages/actions-proto-go/runner/v1/runnerv1connect"
)

// A Client manages communication with the runner.
//
//go:generate mockery --name Client
type Client interface {
	pingv1connect.PingServiceClient
	runnerv1connect.RunnerServiceClient
	Address() string
	Insecure() bool
}
