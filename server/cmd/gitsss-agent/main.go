// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "probe" {
		fmt.Fprintln(os.Stderr, "usage: gitsss-agent probe --server <url> --token <token>")
		os.Exit(2)
	}

	flags := flag.NewFlagSet("probe", flag.ExitOnError)
	serverURL := flags.String("server", "", "gitsss API base URL")
	token := flags.String("token", "", "agent token")
	flags.Parse(os.Args[2:])
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
