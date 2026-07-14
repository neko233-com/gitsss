// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/neko233-com/gitsss/server/internal/agentjob"
	"github.com/neko233-com/gitsss/server/internal/api"
	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
	"github.com/neko233-com/gitsss/server/internal/notification"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/state"
	"github.com/neko233-com/gitsss/server/internal/storage"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func main() {
	token := os.Getenv("GITSSS_API_TOKEN")
	dataPath := os.Getenv("GITSSS_DATA_PATH")
	if dataPath == "" {
		dataPath = "data/gitsss.db"
	}
	stateStore, err := state.Open(dataPath)
	if err != nil {
		log.Fatal(err)
	}
	defer stateStore.Close()
	masterKeyPath := os.Getenv("GITSSS_MASTER_KEY_PATH")
	if masterKeyPath == "" {
		masterKeyPath = dataPath + ".key"
	}
	vault, err := state.OpenVault(masterKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	logStore, err := audit.NewPersistentLog(stateStore, vault)
	if err != nil {
		log.Fatal(err)
	}
	verifier, err := auth.NewPersistentVerifier(stateStore)
	if err != nil {
		log.Fatal(err)
	}
	bootstrapScopes := []string{"auth:read", "auth:write", "dashboard:read", "events:write", "storage:read", "storage:write", "runners:read", "runners:write", "repositories:read", "repositories:search", "notifications:read", "notifications:write", "notifications:dispatch", "takeover:read", "takeover:write", "agents:read", "agents:dispatch", "agents:work"}
	now := time.Now().UTC()
	seeded, err := verifier.EnsureBootstrap(token, now, now.Add(24*time.Hour), bootstrapScopes...)
	if err != nil {
		log.Fatal("GITSSS_API_TOKEN with at least 24 bytes is required for the first startup")
	}
	if seeded {
		log.Print("created a 24-hour bootstrap token; issue a named replacement before it expires")
	}
	dashboardService, err := dashboard.NewPersistentService(6, []dashboard.Event{
		{ID: "merge", Action: "feature/export 合并到 main", Actor: "张晨", At: "10:24", Kind: "change"},
		{ID: "build", Action: "构建 #1942 失败（支付服务）", Actor: "CI", At: "10:18", Kind: "build"},
		{ID: "review", Action: "新建 PR #482：修复导出报表错误", Actor: "李明", At: "09:52", Kind: "review"},
	}, logStore, stateStore, vault)
	if err != nil {
		log.Fatal(err)
	}
	storageService := storage.NewService(4<<40, []storage.Usage{
		{Category: storage.CategoryGitObjects, Bytes: 410 << 30},
		{Category: storage.CategoryLFSAssets, Bytes: 2310 << 30},
		{Category: storage.CategoryArtifacts, Bytes: 250 << 30},
		{Category: storage.CategoryBackups, Bytes: 150 << 30},
	})
	runnerService := runner.NewService("0.1.0")
	notificationService, err := notification.NewPersistentService(notification.NewFactory(), logStore, stateStore, vault)
	if err != nil {
		log.Fatal(err)
	}
	takeoverRegistry, err := takeover.NewPersistentRegistry(logStore, stateStore)
	if err != nil {
		log.Fatal(err)
	}
	agentJobs, err := agentjob.NewPersistentQueue(logStore, stateStore)
	if err != nil {
		log.Fatal(err)
	}
	repositoryPath := os.Getenv("GITSSS_REPOSITORY_PATH")
	if repositoryPath == "" {
		repositoryPath = "."
	}
	repositoryService, err := repository.NewService([]repository.Repository{{ID: "gitsss", Name: "gitsss", DefaultBranch: "main", Path: repositoryPath}})
	if err != nil {
		log.Fatal(err)
	}
	address := os.Getenv("GITSSS_HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8787"
	}
	log.Printf("gitsss API listening on %s", address)
	log.Fatal(http.ListenAndServe(address, api.NewServer(verifier, api.Dependencies{Dashboard: dashboardService, Notifications: notificationService, Repositories: repositoryService, Runners: runnerService, Storage: storageService, Takeovers: takeoverRegistry, Jobs: agentJobs, Audit: logStore}).Handler()))
}
