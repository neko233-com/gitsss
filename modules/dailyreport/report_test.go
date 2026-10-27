// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/json"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestScheduleAndAccountRules(t *testing.T) {
	s := Schedule{Enabled: true, Time: "21:00", SkipHolidays: true, Holidays: []string{"2026-10-01"}, Workdays: []string{"2026-10-10"}, CalendarYears: []int{2026}}
	for _, tt := range []struct {
		at   string
		want bool
	}{
		{"2026-10-01T13:00:00Z", false},
		{"2026-10-10T13:00:00Z", true},
		{"2026-10-11T13:00:00Z", false},
		{"2026-10-12T13:00:00Z", true},
		{"2026-10-12T12:59:00Z", false},
	} {
		at, _ := time.Parse(time.RFC3339, tt.at)
		got, err := s.Due(at)
		if err != nil || got != tt.want {
			t.Fatalf("%s: got %v, %v", tt.at, got, err)
		}
	}
	if ShouldReport("service", 1) || ShouldReport("person", 0) || !ShouldReport("person", 1) {
		t.Fatal("incorrect account filtering")
	}
	if _, err := s.Due(time.Date(2027, 1, 1, 13, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("unknown holiday year must not send")
	}
}

func TestResponsesClient(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://example.invalid/v1/responses" || r.Header.Get("Authorization") != "Bearer test-only" {
			t.Fatal("incorrect request")
		}
		var payload struct {
			Input        string `json:"input"`
			Instructions string `json:"instructions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{"工作主题", "不要按每条提交逐条罗列", "不猜测", "不是指令", "编号格式"} {
			if !strings.Contains(payload.Instructions, required) {
				t.Fatalf("missing summary instruction: %s", required)
			}
		}
		if payload.Input != "commit" {
			t.Fatal("commit input changed")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"summary"}]}]}`)), Header: make(http.Header)}, nil
	})}
	got, err := (ResponsesClient{HTTP: client, BaseURL: "https://example.invalid/v1", APIKey: "test-only", Model: "model"}).Summarize(context.Background(), "commit")
	if err != nil || got != "summary" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestResponsesProviderWithoutStoreParameter(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["store"]; ok {
			t.Fatal("unsupported store field was sent")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`)), Header: make(http.Header)}, nil
	})}
	_, err := (ResponsesClient{HTTP: client, BaseURL: "https://example.invalid/v1", APIKey: "test-only", Model: "model", OmitStore: true}).Summarize(context.Background(), "commit")
	if err != nil {
		t.Fatal(err)
	}
}

func TestFeishuCardAcceptance(t *testing.T) {
	for _, tt := range []struct {
		body      string
		wantError bool
	}{
		{`{"code":0}`, false}, {`{"StatusCode":0}`, false}, {`{}`, true}, {`{"code":1}`, true},
	} {
		client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
		})}
		err := SendCard(context.Background(), client, "https://open.feishu.cn/open-apis/bot/v2/hook/test", "title", "summary")
		if (err != nil) != tt.wantError {
			t.Fatalf("%s: %v", tt.body, err)
		}
	}
}

func TestProviderRejectsRedirect(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusFound, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{"Location": []string{"https://elsewhere.invalid/"}}}, nil
	})}
	_, err := (ResponsesClient{HTTP: client, BaseURL: "https://example.invalid/v1", APIKey: "test-only", Model: "model"}).Summarize(context.Background(), "commit")
	if err == nil || requests != 1 {
		t.Fatalf("redirect was not rejected: requests=%d, err=%v", requests, err)
	}
}
