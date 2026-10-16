// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package listenport

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/neko233-com/gitsss/modules/graceful"
	"github.com/neko233-com/gitsss/modules/setting"
)

// Apply rebinds plain HTTP onto the new port and stores the public URL.
// The new socket is opened before the old one is closed. A failed config
// write binds the previous port again.
func Apply(rawPort string) error {
	port, err := ValidatePort(rawPort)
	if err != nil {
		return err
	}
	if setting.Protocol != setting.HTTP {
		return errors.New("live port changes are available for HTTP; HTTPS still needs a restart")
	}
	nextURL, err := RewriteRootURL(setting.AppURL, port)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(setting.HTTPAddr, port)
	if err := graceful.RebindWeb("tcp", address); err != nil {
		return fmt.Errorf("bind %s: %w", address, err)
	}
	oldPort := setting.HTTPPort
	if err := save(port, nextURL); err != nil {
		_ = graceful.RebindWeb("tcp", net.JoinHostPort(setting.HTTPAddr, oldPort))
		return err
	}
	setting.HTTPPort = port
	setting.AppURL = nextURL
	return nil
}

// AddAlias keeps the current listener open, adds another HTTP port, and makes
// that port the public URL for clone and web links.
func AddAlias(rawPort string) error {
	port, err := ValidatePort(rawPort)
	if err != nil {
		return err
	}
	if setting.Protocol != setting.HTTP {
		return errors.New("live port aliases are available for HTTP; HTTPS still needs a restart")
	}
	if port == setting.HTTPPort {
		return fmt.Errorf("port %s is already the primary HTTP listener", port)
	}
	nextURL, err := RewriteRootURL(setting.AppURL, port)
	if err != nil {
		return err
	}
	for _, current := range setting.HTTPAdditionalPorts {
		if current == port {
			if setting.AppURL == nextURL {
				return nil
			}
			if err := saveAdditional(setting.HTTPAdditionalPorts, nextURL); err != nil {
				return err
			}
			setting.AppURL = nextURL
			return nil
		}
	}
	address := net.JoinHostPort(setting.HTTPAddr, port)
	if err := graceful.AddWebListener("tcp", address); err != nil {
		return fmt.Errorf("bind additional HTTP port %s: %w", port, err)
	}
	additionalPorts := append(append([]string(nil), setting.HTTPAdditionalPorts...), port)
	if err := saveAdditional(additionalPorts, nextURL); err != nil {
		removeErr := graceful.RemoveWebListener(address)
		if removeErr != nil {
			return fmt.Errorf("save HTTP port configuration: %w; rollback listener: %v", err, removeErr)
		}
		return err
	}
	setting.HTTPAdditionalPorts = additionalPorts
	setting.AppURL = nextURL
	return nil
}

func save(port, rootURL string) error {
	if setting.CfgProvider == nil {
		return errors.New("config is not loaded")
	}
	saving, err := setting.CfgProvider.PrepareSaving()
	if err != nil {
		return err
	}
	setKeys(saving, port, rootURL)
	if err := saving.Save(); err != nil {
		return err
	}
	setKeys(setting.CfgProvider, port, rootURL)
	return nil
}

func saveAdditional(ports []string, rootURL string) error {
	if setting.CfgProvider == nil {
		return errors.New("config is not loaded")
	}
	saving, err := setting.CfgProvider.PrepareSaving()
	if err != nil {
		return err
	}
	setAdditionalKeys(saving, ports, rootURL)
	if err := saving.Save(); err != nil {
		return err
	}
	setAdditionalKeys(setting.CfgProvider, ports, rootURL)
	return nil
}

func setKeys(cfg setting.ConfigProvider, port, rootURL string) {
	section := cfg.Section("server")
	writeKey(section, "HTTP_PORT", port)
	writeKey(section, "ROOT_URL", rootURL)
}

func setAdditionalKeys(cfg setting.ConfigProvider, ports []string, rootURL string) {
	section := cfg.Section("server")
	writeKey(section, "HTTP_ADDITIONAL_PORTS", strings.Join(ports, ","))
	writeKey(section, "ROOT_URL", rootURL)
}

func writeKey(section setting.ConfigSection, name, value string) {
	if section.HasKey(name) {
		section.Key(name).SetValue(value)
		return
	}
	if _, err := section.NewKey(name, value); err != nil {
		section.Key(name).SetValue(value)
	}
}
