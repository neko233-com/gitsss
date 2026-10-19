// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"strings"

	"github.com/neko233-com/gitsss/modules/glob"
	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"
)

func notificationEventMatches(settings NotificationSettings, event webhook_module.HookEventType, payload []byte) bool {
	if (settings.EventPolicy == "" || settings.EventPolicy == "all") && settings.TagFilter == "" {
		return true
	}
	data := map[string]any{}
	if json.Unmarshal(payload, &data) != nil {
		return false
	}
	tag := ""
	if event == webhook_module.HookEventRelease {
		release := notificationMap(data["release"])
		tag = notificationString(release["tag_name"])
		if settings.EventPolicy == "formal_release" && (tag == "" || notificationString(data["action"]) != "published" || release["draft"] != false || release["prerelease"] != false) {
			return false
		}
	} else if event == webhook_module.HookEventCreate && notificationString(data["ref_type"]) == "tag" {
		tag = strings.TrimPrefix(notificationString(data["ref"]), "refs/tags/")
	} else if settings.EventPolicy != "" && settings.EventPolicy != "all" {
		return false
	} else if ref := notificationString(data["ref"]); strings.HasPrefix(ref, "refs/tags/") {
		tag = strings.TrimPrefix(ref, "refs/tags/")
	}
	if settings.EventPolicy == "formal_release" && event != webhook_module.HookEventRelease {
		return false
	}
	if settings.EventPolicy == "tag_created" && (event != webhook_module.HookEventCreate || tag == "") {
		return false
	}
	if settings.TagFilter != "" {
		matcher, err := glob.Compile(settings.TagFilter)
		return err == nil && tag != "" && matcher.Match(tag)
	}
	return true
}
