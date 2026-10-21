// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hookrelay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json" //nolint:depguard // Keep the standalone hook tests on the same standard-library codec.
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, mode string) map[string]string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, mode+".d"), 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(filepath.Join(dir, mode+".d", "gitea"), data, 0o700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return map[string]string{
		EnvPrefix + "ORIGINAL_DIR": dir, EnvPrefix + "SHA_" + strings.ToUpper(strings.ReplaceAll(mode, "-", "_")): hex.EncodeToString(digest[:]),
		EnvPrefix + "TOKEN": "fixture-capability", EnvPrefix + "PROC_RECEIVE": "true", "SSH_ORIGINAL_COMMAND": "receive-pack",
		"GITEA_REPO_USER_NAME": "fixture", "GITEA_REPO_NAME": "fixture", "GITEA_PUSHER_ID": "17",
		"GITEA_PUSHER_NAME": "fixture", "GITEA_PR_ID": "23", "GITEA_DEPLOY_KEY_ID": "29", "GITEA_ACTIONS_TASK_ID": "31",
		"GITEA_PUSH_TRIGGER": "pr-merge-to-base", "GIT_OBJECT_DIRECTORY": "objects/incoming-fixture",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": "objects", "GIT_QUARANTINE_PATH": "objects/incoming-fixture",
		"GIT_PUSH_OPTION_COUNT": "3", "GIT_PUSH_OPTION_0": "key=first", "GIT_PUSH_OPTION_1": "ignored", "GIT_PUSH_OPTION_2": "key=second=value",
	}
}

func TestLocalURL(t *testing.T) {
	for raw, expected := range map[string]bool{
		"http://127.0.0.1:3000/": true, "http://[::1]:3000/sub/": true, "http://localhost:3000/": true,
		"https://127.0.0.1/": false, "http://example.invalid/": false, "http://user@127.0.0.1/": false,
		"http://127.0.0.1/?token=x": false, "http://127.0.0.1/#fragment": false, "file:///tmp/socket": false,
	} {
		if LocalURL(raw) != expected {
			t.Errorf("LocalURL(%q) != %v", raw, expected)
		}
	}
}

func TestPreReceiveBatchesAndWireOptions(t *testing.T) {
	env := fixture(t, "pre-receive")
	var batches []Options
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/internal/hook/pre-receive/fixture/fixture" || r.Header.Get("X-Gitea-Internal-Auth") != "Bearer fixture-capability" {
			t.Error("hook request lost its original API/authentication binding")
		}
		var options Options
		if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
			t.Error(err)
		}
		batches = append(batches, options)
		_, _ = io.WriteString(w, "success")
	}))
	defer server.Close()
	env[EnvPrefix+"URL"] = server.URL
	var input strings.Builder
	for index := range 501 {
		fmt.Fprintf(&input, "%040x %040x refs/heads/branch-%d\n", index, index+1, index)
	}
	var output, errors bytes.Buffer
	if code := Run(context.Background(), "pre-receive", nil, strings.NewReader(input.String()), &output, &errors, func(key string) string { return env[key] }); code != 0 {
		t.Fatalf("relay failed: %d %s", code, errors.String())
	}
	if len(batches) != 2 || len(batches[0].RefFullNames) != 500 || len(batches[1].RefFullNames) != 1 || batches[1].RefFullNames[0] != "refs/heads/branch-500" {
		t.Fatal("the original 500-ref batch boundary or input order changed")
	}
	value := batches[0]
	if value.UserID != 17 || value.PullRequestID != 23 || value.DeployKeyID != 29 || value.ActionsTaskID != 31 || value.GitPushOptions["key"] != "second=value" || value.GitQuarantinePath != "objects/incoming-fixture" {
		t.Fatal("authorization, push options or quarantine environment was not preserved")
	}
	if output.Len() != 0 || errors.Len() != 0 {
		t.Fatal("non-verbose accepted hooks must remain silent")
	}
}

func TestHookRejectAndRedirectDoNotLeakCredentials(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		t.Run(strconv.FormatBool(redirect), func(t *testing.T) {
			env := fixture(t, "pre-receive")
			otherCalls := 0
			other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { otherCalls++ }))
			defer other.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if redirect {
					http.Redirect(w, r, other.URL, http.StatusFound)
					return
				}
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `{"user_msg":"Protected branch rejected","err":"private diagnostic fixture-capability"}`)
			}))
			defer server.Close()
			env[EnvPrefix+"URL"] = server.URL
			var output, errors bytes.Buffer
			code := Run(context.Background(), "pre-receive", nil, strings.NewReader("old new refs/heads/main\n"), &output, &errors, func(key string) string { return env[key] })
			if code != 1 || otherCalls != 0 || strings.Contains(errors.String(), "fixture-capability") || strings.Contains(errors.String(), "private diagnostic") {
				t.Fatal("rejection or internal-credential containment failed")
			}
			if !redirect && !strings.Contains(errors.String(), "Protected branch rejected") {
				t.Fatal("the server's public rejection was lost")
			}
		})
	}
}

func TestChangedDelegateFallsBackBeforeAPI(t *testing.T) {
	env := fixture(t, "pre-receive")
	if err := os.WriteFile(filepath.Join(env[EnvPrefix+"ORIGINAL_DIR"], "pre-receive.d", "gitea"), []byte("custom handler\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if code := Run(context.Background(), "pre-receive", nil, strings.NewReader("old new refs/heads/main\n"), io.Discard, io.Discard, func(key string) string { return env[key] }); code != DelegateChanged {
		t.Fatal("changed managed hook was not handed back to the original shell")
	}
}

func TestUpdatePullAndInternalSemantics(t *testing.T) {
	env := fixture(t, "update")
	for _, ref := range []string{"refs/pull/1/head", "refs/pull/1/merge", "refs/pull/1", "refs/heads/main"} {
		for _, internal := range []string{"false", "true"} {
			env["GITEA_INTERNAL_PUSH"] = internal
			code := Run(context.Background(), "update", []string{ref, "old", "new"}, strings.NewReader(""), io.Discard, io.Discard, func(key string) string { return env[key] })
			want := 0
			if strings.HasPrefix(ref, "refs/pull/") && strings.Contains(ref[len("refs/pull/"):], "/") && internal == "false" {
				want = 1
			}
			if code != want {
				t.Fatalf("ref=%s internal=%s: %d != %d", ref, internal, code, want)
			}
		}
	}
}

func TestPostReceiveOrderResultsAndWiki(t *testing.T) {
	for _, wiki := range []string{"false", "true"} {
		env := fixture(t, "post-receive")
		env["GITEA_REPO_IS_WIKI"] = wiki
		updated, called := false, false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			var options Options
			_ = json.NewDecoder(r.Body).Decode(&options)
			if !updated || options.UserName != "fixture" || options.PushTrigger != "pr-merge-to-base" || options.DeployKeyID != 0 || options.ActionsTaskID != 0 {
				t.Error("post-receive call order or actor semantics changed")
			}
			_, _ = io.WriteString(w, `{"Results":[{"Message":true,"Create":true,"Branch":"feature","URL":"http://example.invalid/pr"}]}`)
		}))
		env[EnvPrefix+"URL"] = server.URL
		var output, errors bytes.Buffer
		code := run(context.Background(), "post-receive", nil, strings.NewReader("old new refs/heads/main\n"), &output, &errors, func(key string) string { return env[key] }, func(context.Context) error { updated = true; return nil })
		server.Close()
		if code != 0 || !updated || called != (wiki == "false") || (wiki == "false" && !strings.Contains(errors.String(), "Create a new pull request for 'feature':")) {
			t.Fatal("post-receive database call, trailer or wiki behavior changed")
		}
	}
}

func TestProgressDelayAndChildEnvironment(t *testing.T) {
	var output bytes.Buffer
	writer := &progressWriter{output: &output, start: time.Now(), delay: time.Hour}
	_, _ = writer.Write([]byte("buffered"))
	writer.Close()
	if output.Len() != 0 {
		t.Fatal("fast hooks should suppress delayed progress")
	}
	writer.start = time.Now().Add(-2 * time.Hour)
	_, _ = writer.Write([]byte("visible"))
	if output.String() != "bufferedvisible" {
		t.Fatal("slow hooks lost the buffered progress prefix")
	}
	got := PublicEnvironment([]string{"PATH=fixture", EnvPrefix + "TOKEN=fixture-capability", EnvPrefix + "URL=http://localhost/", "GITEA_PUSHER_ID=17"})
	if strings.Join(got, ";") != "PATH=fixture;GITEA_PUSHER_ID=17" {
		t.Fatal("new internal credentials escaped into a child command")
	}
}
