// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/dailyreport"
)

// reportFailureMessage deliberately excludes error details that could contain secrets.
func reportFailureMessage(orgID int64, date time.Time, mode string, result dailyreport.RunResult, err error) string {
	reason := "执行异常，请在 GitSSS 系统提示或服务日志中查看详情。"
	message := err.Error()
	switch {
	case strings.Contains(message, "responses provider returned HTTP 401") || strings.Contains(message, "responses provider returned HTTP 403"):
		reason = "模型服务认证失败，请检查 API Key 和服务地址。"
	case strings.Contains(message, "responses provider returned HTTP 402"):
		reason = "模型服务余额或配额不足，请检查服务账户。"
	case strings.Contains(message, "responses provider returned HTTP 429"):
		reason = "模型服务请求受限，请检查配额或稍后重试。"
	case strings.Contains(message, "responses provider") || strings.Contains(message, "responses request"):
		reason = "模型服务请求或返回结果异常，请检查服务日志。"
	case strings.Contains(message, "Feishu") || strings.Contains(message, "send card"):
		reason = "飞书卡片发送失败，请检查机器人配置和服务日志。"
	case strings.Contains(message, "holiday calendar"):
		reason = "当前年份的节假日日历尚未维护。"
	case strings.Contains(message, "push activity") || strings.Contains(message, "push repository") || strings.Contains(message, "complete push"):
		reason = "提交记录不完整，日报未继续发送，请检查服务日志。"
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}
	return fmt.Sprintf("组织 ID：%d\n日期：%s（北京时间）\n触发方式：%s\n已发送：%d，已跳过：%d，失败：%d\n原因：%s", orgID, date.In(loc).Format("2006-01-02"), mode, result.Sent, result.Skipped, result.Failed, reason)
}

func notifyReportFailure(ctx context.Context, webhook string, orgID int64, date time.Time, mode string, result dailyreport.RunResult, runErr error) error {
	return notifyReportFailureWithSender(ctx, webhook, orgID, date, mode, result, runErr, func(ctx context.Context, webhook, message string) error {
		return dailyreport.SendCard(ctx, &http.Client{Timeout: 10 * time.Second}, webhook, "日报 Agent 执行失败", message)
	})
}

func notifyReportFailureWithSender(ctx context.Context, webhook string, orgID int64, date time.Time, mode string, result dailyreport.RunResult, runErr error, send func(context.Context, string, string) error) error {
	if runErr == nil || webhook == "" {
		return runErr
	}
	alertCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	alert := reportFailureMessage(orgID, date, mode, result, runErr)
	if err := send(alertCtx, webhook, alert); err != nil {
		return errors.Join(runErr, fmt.Errorf("Feishu failure alert: %w", err))
	}
	return runErr
}
