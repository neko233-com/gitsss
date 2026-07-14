// Copyright 2026 gitsss Authors. All rights reserved.

package auth

import (
	"errors"
	"sort"
	"time"
)

var ErrUnknownRole = errors.New("unknown authorization role")

type RoleTemplate struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Purpose         string        `json:"purpose"`
	Scopes          []string      `json:"scopes"`
	DefaultDuration time.Duration `json:"-"`
	MaximumDuration time.Duration `json:"-"`
	DefaultHours    int           `json:"default_hours"`
	MaximumHours    int           `json:"maximum_hours"`
	Warnings        []string      `json:"warnings"`
}

var roleTemplates = map[string]RoleTemplate{
	"administrator": newRole("administrator", "管理员", "配置实例、授权和高风险运维", 8*time.Hour, 24*time.Hour,
		[]string{"auth:read", "auth:write", "dashboard:read", "events:write", "storage:read", "storage:write", "runners:read", "runners:write", "repositories:read", "repositories:write", "repositories:search", "repositories:clone", "repositories:push", "notifications:read", "notifications:write", "notifications:dispatch", "takeover:read", "takeover:write", "agents:read", "agents:dispatch"},
		"可创建和撤销授权，并能执行接管等高风险操作；建议仅短时签发。"),
	"maintainer": newRole("maintainer", "维护者", "管理仓库、交付和团队通知", 7*24*time.Hour, 30*24*time.Hour,
		[]string{"dashboard:read", "events:write", "storage:read", "runners:read", "repositories:read", "repositories:write", "repositories:search", "repositories:clone", "repositories:push", "notifications:read", "notifications:write"}),
	"contributor": newRole("contributor", "协作者", "浏览、搜索、克隆并推送代码", 30*24*time.Hour, 90*24*time.Hour,
		[]string{"dashboard:read", "repositories:read", "repositories:search", "repositories:clone", "repositories:push"}),
	"viewer": newRole("viewer", "只读成员", "查看大盘、浏览、搜索和克隆仓库", 30*24*time.Hour, 90*24*time.Hour,
		[]string{"dashboard:read", "repositories:read", "repositories:search", "repositories:clone"}),
	"migration-agent": newRole("migration-agent", "接管 Agent", "领取并执行限定的接管和备份任务", 24*time.Hour, 7*24*time.Hour,
		[]string{"agents:work", "takeover:read", "takeover:write", "storage:read"}, "仅用于指定旧服务器；接管结束后立即撤销。"),
	"runner-agent": newRole("runner-agent", "Runner Agent", "报告 Runner 健康状态并领取受控任务", 7*24*time.Hour, 30*24*time.Hour,
		[]string{"agents:work", "runners:write", "storage:read"}),
}

func Roles() []RoleTemplate {
	roles := make([]RoleTemplate, 0, len(roleTemplates))
	for _, template := range roleTemplates {
		roles = append(roles, cloneRole(template))
	}
	sort.Slice(roles, func(left, right int) bool { return roles[left].ID < roles[right].ID })
	return roles
}

func Role(id string) (RoleTemplate, error) {
	template, ok := roleTemplates[id]
	if !ok {
		return RoleTemplate{}, ErrUnknownRole
	}
	return cloneRole(template), nil
}

func newRole(id, name, purpose string, defaultDuration, maximumDuration time.Duration, scopes []string, warnings ...string) RoleTemplate {
	return RoleTemplate{
		ID: id, Name: name, Purpose: purpose, Scopes: scopes,
		DefaultDuration: defaultDuration, MaximumDuration: maximumDuration,
		DefaultHours: int(defaultDuration / time.Hour), MaximumHours: int(maximumDuration / time.Hour),
		Warnings: warnings,
	}
}

func cloneRole(template RoleTemplate) RoleTemplate {
	template.Scopes = append([]string(nil), template.Scopes...)
	template.Warnings = append([]string(nil), template.Warnings...)
	return template
}

func validRoleID(id string) bool {
	if id == "" {
		return true
	}
	_, ok := roleTemplates[id]
	return ok
}
