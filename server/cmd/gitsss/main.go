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
	verifier := auth.NewVerifier(auth.NewRecord("bootstrap", token, time.Time{}, "dashboard:read", "events:write"))

	address := os.Getenv("GITSSS_HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8787"
	}
	log.Printf("gitsss API listening on %s", address)
	log.Fatal(http.ListenAndServe(address, api.NewServer(verifier, dashboardService).Handler()))
}
