// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/dailyreport"
)

func TestReportFailureMessageRedactsErrorAndIdentifiesFailures(t *testing.T) {
	at := time.Date(2026, 11, 4, 13, 0, 0, 0, time.UTC)
	result := dailyreport.RunResult{Sent: 1, Skipped: 2, Failed: 3}
	for _, tc := range []struct{ err, want string }{
		{"responses provider returned HTTP 401 secret-value", "模型服务认证失败"},
		{"responses provider returned HTTP 402 secret-value", "余额或配额不足"},
		{"responses provider returned HTTP 429 secret-value", "请求受限"},
		{"send card: secret-value", "飞书卡片发送失败"},
		{"holiday calendar is not configured for this year secret-value", "节假日日历"},
		{"unexpected secret-value", "执行异常"},
	} {
		got := reportFailureMessage(2, at, "定时任务", result, errors.New(tc.err))
		for _, part := range []string{tc.want, "2026-11-04", "定时任务", "已发送：1，已跳过：2，失败：3"} {
			if !strings.Contains(got, part) {
				t.Fatalf("message missing %q: %q", part, got)
			}
		}
		if strings.Contains(got, "secret-value") {
			t.Fatal("error detail leaked to alert")
		}
	}
}

func TestNotifyReportFailure(t *testing.T) {
	problem := errors.New("responses provider returned HTTP 401 secret-value")
	count := 0
	send := func(ctx context.Context, webhook, message string) error {
		count++
		if ctx.Err() != nil || webhook != "webhook" || !strings.Contains(message, "模型服务认证失败") || strings.Contains(message, "secret-value") {
			t.Fatal("invalid failure alert")
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := notifyReportFailureWithSender(ctx, "webhook", 2, time.Now(), "定时任务", dailyreport.RunResult{Failed: 1}, problem, send)
	if !errors.Is(got, problem) || count != 1 {
		t.Fatal("failure alert was not sent or original error was lost")
	}
	if got := notifyReportFailureWithSender(ctx, "webhook", 2, time.Now(), "定时任务", dailyreport.RunResult{}, nil, send); got != nil || count != 1 {
		t.Fatal("successful run must not send an alert")
	}
	sendFailed := errors.New("notification failed")
	got = notifyReportFailureWithSender(ctx, "webhook", 2, time.Now(), "定时任务", dailyreport.RunResult{}, problem, func(context.Context, string, string) error { return sendFailed })
	if !errors.Is(got, problem) || !errors.Is(got, sendFailed) {
		t.Fatal("alert failure must remain visible alongside the original failure")
	}
}
