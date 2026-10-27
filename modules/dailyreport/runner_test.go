// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type memoryDelivery map[string]bool

func (m memoryDelivery) Delivered(date, account string) (bool, error) {
	return m[date+":"+account], nil
}

func TestReportPipelineSeparatesPeopleAndSkipsServiceAccounts(t *testing.T) {
	pushes := []Push{{RepositoryID: 1, Total: 3, Commits: []Commit{
		{ID: "a", AuthorEmail: "a@example.invalid", Message: "work A"},
		{ID: "b", AuthorEmail: "b@example.invalid", Message: "work B"},
		{ID: "c", AuthorEmail: "ci@example.invalid", Message: "build"},
	}}}
	grouped, err := GroupCommits(pushes, map[string]string{"a": "person", "b": "person", "ci": "service"}, func(email string) (string, error) {
		return strings.Split(email, "@")[0], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var providerCalls, cardCalls int
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"code":0}`
		if req.URL.Host == "provider.invalid" {
			providerCalls++
			body = `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"fixture summary"}]}]}`
		} else if req.URL.Host == "open.feishu.cn" {
			cardCalls++
		} else {
			t.Fatalf("unexpected host: %s", req.URL.Host)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	provider := ResponsesClient{HTTP: client, BaseURL: "https://provider.invalid/v1", APIKey: "fixture", Model: "fixture"}
	store := memoryDelivery{}
	run := func() (RunResult, error) {
		return Run(context.Background(), "2026-01-01", grouped, store, provider.Summarize, func(ctx context.Context, account, summary string) error {
			return SendCard(ctx, client, "https://open.feishu.cn/open-apis/bot/v2/hook/fixture", account, summary)
		})
	}
	result, err := run()
	if err != nil || result.Sent != 2 || providerCalls != 2 || cardCalls != 2 {
		t.Fatalf("first run: %+v %v, provider=%d cards=%d", result, err, providerCalls, cardCalls)
	}
	result, err = run()
	if err != nil || result.Skipped != 2 || providerCalls != 2 || cardCalls != 2 {
		t.Fatalf("retry: %+v %v, provider=%d cards=%d", result, err, providerCalls, cardCalls)
	}
}

func (m memoryDelivery) MarkDelivered(date, account string) error {
	m[date+":"+account] = true
	return nil
}

func TestRunRetriesOnlyUnsentCards(t *testing.T) {
	store := memoryDelivery{}
	grouped := map[string][]AttributedCommit{"a": {{ID: "1"}}, "b": {{ID: "2"}}, "empty": {}}
	fail := true
	summarize := func(context.Context, string) (string, error) { return "summary", nil }
	send := func(_ context.Context, account, _ string) error {
		if account == "b" && fail {
			return errors.New("temporary")
		}
		return nil
	}
	first, err := Run(context.Background(), "2026-01-01", grouped, store, summarize, send)
	if err == nil || first.Sent != 1 || first.Failed != 1 {
		t.Fatalf("first run: %+v %v", first, err)
	}
	fail = false
	second, err := Run(context.Background(), "2026-01-01", grouped, store, summarize, send)
	if err != nil || second.Sent != 1 || second.Skipped != 1 {
		t.Fatalf("retry: %+v %v", second, err)
	}
}
