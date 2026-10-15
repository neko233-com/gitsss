// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/neko233-com/gitsss/modules/setting"
)

var errConfigNotLoaded = errors.New("config is not loaded")

// Settings is the persisted [cron.update_checker] mirror configuration.
// A nil Enabled pointer leaves the existing schedule flag untouched.
type Settings struct {
	Mirrors     []string
	GitHubRepo  string
	CheckGitHub bool
	Enabled     *bool
}

// OptionsFromConfig reads the update checker section from the loaded config.
// CHECK_GITHUB stays false when the key is absent, so startup does not contact GitHub.
func OptionsFromConfig() Options {
	opts := Options{}
	if setting.CfgProvider == nil {
		return opts
	}
	section := setting.CfgProvider.Section("cron.update_checker")
	if section.HasKey("HTTP_ENDPOINT") {
		opts.HTTPEndpoint = strings.TrimSpace(section.Key("HTTP_ENDPOINT").String())
	}
	if section.HasKey("CHECK_GITHUB") {
		opts.CheckGitHub = section.Key("CHECK_GITHUB").MustBool(false)
	}
	if section.HasKey("GITHUB_REPO") {
		opts.GitHubRepo = strings.TrimSpace(section.Key("GITHUB_REPO").String())
	}
	if section.HasKey("MIRRORS") {
		mirrors, err := ParseMirrors(section.Key("MIRRORS").String())
		if err == nil {
			opts.Mirrors = mirrors
		}
	}
	if section.HasKey("PROBE_TIMEOUT") {
		opts.ProbeTimeout = section.Key("PROBE_TIMEOUT").MustDuration(0)
	}
	return opts
}

// SaveSettings writes mirror settings back to app.ini and the live config.
// HTTP_ENDPOINT is preserved.
func SaveSettings(settings Settings) error {
	if setting.CfgProvider == nil {
		return errConfigNotLoaded
	}
	saving, err := setting.CfgProvider.PrepareSaving()
	if err != nil {
		return err
	}
	if err := applySettings(saving, settings); err != nil {
		return err
	}
	if err := saving.Save(); err != nil {
		return err
	}
	return applySettings(setting.CfgProvider, settings)
}

func applySettings(cfg setting.ConfigProvider, settings Settings) error {
	section := cfg.Section("cron.update_checker")
	repo := strings.TrimSpace(settings.GitHubRepo)
	if repo == "" {
		repo = DefaultGitHubRepo
	}
	if err := validateRepo(repo); err != nil {
		return err
	}
	if len(settings.Mirrors) > maxMirrors {
		return fmt.Errorf("at most %d update mirrors are allowed", maxMirrors)
	}
	for _, mirror := range settings.Mirrors {
		if _, err := joinMirror(mirror, "https://api.github.com/"); err != nil {
			return err
		}
	}
	setKey(section, "MIRRORS", strings.Join(settings.Mirrors, ","))
	setKey(section, "GITHUB_REPO", repo)
	setKey(section, "CHECK_GITHUB", boolText(settings.CheckGitHub))
	if settings.Enabled != nil {
		setKey(section, "ENABLED", boolText(*settings.Enabled))
	}
	return nil
}

func setKey(section setting.ConfigSection, name, value string) {
	if section.HasKey(name) {
		section.Key(name).SetValue(value)
		return
	}
	_, _ = section.NewKey(name, value)
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// MirrorText renders mirrors one per line for the admin form.
func MirrorText(mirrors []string) string {
	return strings.Join(mirrors, "\n")
}
