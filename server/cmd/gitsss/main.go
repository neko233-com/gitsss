// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/neko233-com/gitsss/server/internal/api"
	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
	"github.com/neko233-com/gitsss/server/internal/notification"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/storage"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func main() {
	token := os.Getenv("GITSSS_API_TOKEN")
	if token == "" {
		log.Fatal("GITSSS_API_TOKEN must be configured")
	}

	logStore := &audit.Log{}
	dashboardService := dashboard.NewService(6, []dashboard.Event{
		{ID: "merge", Action: "feature/export 合并到 main", Actor: "张晨", At: "10:24", Kind: "change"},
		{ID: "build", Action: "构建 #1942 失败（支付服务）", Actor: "CI", At: "10:18", Kind: "build"},
		{ID: "review", Action: "新建 PR #482：修复导出报表错误", Actor: "李明", At: "09:52", Kind: "review"},
	}, logStore)
	storageService := storage.NewService(4<<40, []storage.Usage{
		{Category: storage.CategoryGitObjects, Bytes: 410 << 30},
		{Category: storage.CategoryLFSAssets, Bytes: 2310 << 30},
		{Category: storage.CategoryArtifacts, Bytes: 250 << 30},
		{Category: storage.CategoryBackups, Bytes: 150 << 30},
	})
	runnerService := runner.NewService("0.1.0")
	notificationService := notification.NewService(notification.NewFactory(), logStore)
	takeoverRegistry := takeover.NewRegistry(logStore)
	repositoryPath := os.Getenv("GITSSS_REPOSITORY_PATH")
	if repositoryPath == "" {
		repositoryPath = "."
	}
	repositoryService, err := repository.NewService([]repository.Repository{{ID: "gitsss", Name: "gitsss", DefaultBranch: "main", Path: repositoryPath}})
	if err != nil {
		log.Fatal(err)
	}
	verifier := auth.NewVerifier(auth.NewRecord("bootstrap", token, time.Time{}, "dashboard:read", "events:write", "storage:read", "storage:write", "runners:read", "runners:write", "repositories:read", "repositories:search", "notifications:read", "notifications:write", "notifications:dispatch", "takeover:read", "takeover:write"))

	address := os.Getenv("GITSSS_HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8787"
	}
	log.Printf("gitsss API listening on %s", address)
	log.Fatal(http.ListenAndServe(address, api.NewServer(verifier, api.Dependencies{Dashboard: dashboardService, Notifications: notificationService, Repositories: repositoryService, Runners: runnerService, Storage: storageService, Takeovers: takeoverRegistry}).Handler()))
}
