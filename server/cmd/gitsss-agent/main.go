// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/runner"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gitsss-agent <probe|runner-report> [options]")
		os.Exit(2)
	}
	if os.Args[1] == "probe" {
		probe(os.Args[2:])
		return
	}
	if os.Args[1] == "runner-report" {
		runnerReport(os.Args[2:])
		return
	}
	fmt.Fprintln(os.Stderr, "usage: gitsss-agent <probe|runner-report> [options]")
	os.Exit(2)
}

func probe(args []string) {
	flags := flag.NewFlagSet("probe", flag.ExitOnError)
	serverURL := flags.String("server", "", "gitsss API base URL")
	token := flags.String("token", "", "agent token")
	flags.Parse(args)
	if strings.TrimSpace(*serverURL) == "" || strings.TrimSpace(*token) == "" {
		fmt.Fprintln(os.Stderr, "--server and --token are required")
		os.Exit(2)
	}

	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(*serverURL, "/")+"/api/v1/dashboard", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	request.Header.Set("Authorization", "Bearer "+*token)
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		fmt.Fprintf(os.Stderr, "probe failed: %s: %s\n", response.Status, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var snapshot struct {
		Metrics []struct {
			Label string `json:"label"`
			Value int    `json:"value"`
		} `json:"metrics"`
	}
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("agent connected: %d dashboard metrics available\n", len(snapshot.Metrics))
}

func runnerReport(args []string) {
	flags := flag.NewFlagSet("runner-report", flag.ExitOnError)
	serverURL := flags.String("server", "", "gitsss API base URL")
	token := flags.String("token", "", "agent token")
	id := flags.String("id", "", "stable agent id")
	platform := flags.String("platform", "", "runner platform, for example windows/amd64")
	version := flags.String("version", "", "installed runner version")
	flags.Parse(args)
	if strings.TrimSpace(*serverURL) == "" || strings.TrimSpace(*token) == "" || strings.TrimSpace(*id) == "" || strings.TrimSpace(*platform) == "" {
		fmt.Fprintln(os.Stderr, "--server, --token, --id and --platform are required")
		os.Exit(2)
	}

	payload, err := json.Marshal(runner.Report{ID: *id, Platform: *platform, Version: *version})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(*serverURL, "/")+"/api/v1/runners/reconcile", bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	request.Header.Set("Authorization", "Bearer "+*token)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		fmt.Fprintf(os.Stderr, "runner report failed: %s: %s\n", response.Status, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var reconciliation runner.Reconciliation
	if err := json.NewDecoder(response.Body).Decode(&reconciliation); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("runner action: %s (%s)\n", reconciliation.Action, reconciliation.Message)
}
