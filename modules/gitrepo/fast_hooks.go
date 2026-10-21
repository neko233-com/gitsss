// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/modules/git"
	"github.com/neko233-com/gitsss/modules/git/gitcmd"
	"github.com/neko233-com/gitsss/modules/hookrelay"
	"github.com/neko233-com/gitsss/modules/repolayout"
	"github.com/neko233-com/gitsss/modules/setting"
)

var (
	relayProbeCache         sync.Map
	relayExtensionCache     sync.Map
	relayFileInitialization sync.Map
)

func availableHookRelay(ctx context.Context) string {
	name := "gitsss-hook"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(filepath.Dir(setting.AppPath), name)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	key := fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().UnixNano())
	if valid, ok := relayProbeCache.Load(key); ok {
		if valid.(bool) {
			return path
		}
		return ""
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(probeCtx, path, "--protocol-version").Output()
	valid := err == nil && strings.TrimSpace(string(output)) == hookrelay.Protocol
	relayProbeCache.Store(key, valid)
	if valid {
		return path
	}
	return ""
}

func regularHookMatches(path, expected string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(expected)) || !checkExecutable(path) {
		return false
	}
	data, err := os.ReadFile(path)
	return err == nil && bytes.Equal(data, []byte(expected))
}

func hookRelayScript(template string, update bool) string {
	template = strings.ReplaceAll(template, "hookname=$(basename $0)", `hookname=$(basename "$0")`)
	template = strings.ReplaceAll(template, "$(dirname $0)/..", "${GITSSS_HOOK_ORIGINAL_DIR}/..")
	template = strings.ReplaceAll(template, "$(dirname $0/..)", "${GITSSS_HOOK_ORIGINAL_DIR}/..")
	template = strings.ReplaceAll(template, "for hook in ${GIT_DIR}/hooks/${hookname}.d/*; do", `for hook in "${GIT_DIR}/hooks/${hookname}.d/"*; do`)
	original := `  echo "${data}" | "${hook}"`
	invoke := `echo "${data}" | "${GITSSS_HOOK_CLIENT}" "${hookname}"`
	fallback := `echo "${data}" | (unset GITSSS_HOOK_TOKEN; "${GITSSS_HOOK_CLIENT}" --run-custom-hook "${hook}")`
	if update {
		original = `  "${hook}" $1 $2 $3`
		invoke = `"${GITSSS_HOOK_CLIENT}" "${hookname}" "$1" "$2" "$3"`
		fallback = `(unset GITSSS_HOOK_TOKEN; "${GITSSS_HOOK_CLIENT}" --run-custom-hook "${hook}" "$1" "$2" "$3")`
	}
	return strings.Replace(template, original+"\n  exitcodes=\"${exitcodes} $?\"", `  if [ "${hook##*/}" = gitea ] && [ -n "${GITSSS_HOOK_TOKEN}" ]; then
    `+invoke+`
    result=$?
    if [ "${result}" -eq 75 ]; then
      `+fallback+`
      result=$?
    fi
  else
    `+fallback+`
    result=$?
  fi
  exitcodes="${exitcodes} ${result}"`, 1)
}

func originalHookRelayScript() string {
	return fmt.Sprintf(`#!/usr/bin/env %s
hook="${GITSSS_HOOK_ORIGINAL_DIR}/$(basename "$0")"
test -x "${hook}" && test -f "${hook}" || exit 0
unset GITSSS_HOOK_TOKEN
exec "${GITSSS_HOOK_CLIENT}" --run-custom-hook "${hook}" "$@"
`, setting.ScriptType)
}

func ensureRelayFile(directory, name, script string) error {
	path := filepath.Join(directory, name)
	if data, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(data, []byte(script)) || !checkExecutable(path) {
			return errors.New("managed receive hook proxy changed")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Only first-time cache installation is serialized, never repository pushes.
	initialize, _ := relayFileInitialization.LoadOrStore(path, &sync.Mutex{})
	lock := initialize.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if data, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(data, []byte(script)) || !checkExecutable(path) {
			return errors.New("managed receive hook proxy changed")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(directory, ".hook-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0o700); err == nil {
		_, err = file.WriteString(script)
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(file.Name(), path); err != nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(data, []byte(script)) {
			return err
		}
	}
	return nil
}

func ensureRelayLink(directory, name, helper string) error {
	path := filepath.Join(directory, name)
	if target, err := os.Readlink(path); err == nil {
		if target == helper {
			return nil
		}
		return errors.New("managed receive hook target changed")
	}
	initialize, _ := relayFileInitialization.LoadOrStore(path, &sync.Mutex{})
	lock := initialize.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if target, err := os.Readlink(path); err == nil && target == helper {
		return nil
	}
	return os.Symlink(helper, path)
}

// PrepareReceiveHookRelay only overlays recognized managed delegates for this command.
// Existing repository files and custom hooksPath values remain authoritative.
func PrepareReceiveHookRelay(ctx context.Context, repo Repository, command *gitcmd.Command) ([]string, error) {
	if setting.Protocol != setting.HTTP || setting.LocalUseProxyProtocol || !hookrelay.LocalURL(setting.LocalURL) || setting.InternalToken == "" {
		return nil, nil
	}
	helper := availableHookRelay(ctx)
	if helper == "" {
		return nil, nil
	}
	_, _, configErr := RunCmdString(ctx, repo, gitcmd.NewCommand("config", "--get", "core.hooksPath"))
	if !gitcmd.IsErrorExitCode(configErr, 1) {
		return nil, nil
	}
	directory := filepath.Join(repoPath(repo), "hooks")
	names, templates, delegates := getHookTemplates()
	for index, name := range names {
		if !regularHookMatches(filepath.Join(directory, name), templates[index]) || delegates[index] != "" && !regularHookMatches(filepath.Join(directory, name+".d", "gitea"), delegates[index]) {
			return nil, nil
		}
	}
	scripts := make(map[string]string)
	for index, name := range names {
		scripts[name] = originalHookRelayScript()
		if name != "proc-receive" {
			scripts[name] = hookRelayScript(templates[index], name == "update")
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if _, managed := scripts[entry.Name()]; managed || entry.IsDir() || !checkExecutable(filepath.Join(directory, entry.Name())) {
			continue
		}
		names = append(names, entry.Name())
		scripts[entry.Name()] = originalHookRelayScript()
	}
	lfsRoot := ""
	if setting.LFS.Storage != nil && setting.LFS.Storage.Type == setting.LocalStorageType {
		lfsRoot = setting.LFS.Storage.Path
	}
	cacheKey := setting.AppDataPath + "\x00" + setting.RepoRootPath + "\x00" + lfsRoot
	initialize, _ := relayExtensionCache.LoadOrStore(cacheKey, sync.OnceValues(func() (string, error) {
		return repolayout.ReserveExtension(setting.AppDataPath, setting.RepoRootPath, lfsRoot, "receive-hooks")
	}))
	extension, err := initialize.(func() (string, error))()
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	direct := runtime.GOOS != "windows"
	if direct {
		content.WriteString(hookrelay.Protocol + "\x00" + helper + "\x00")
	}
	for _, name := range names {
		content.WriteString(name + "\x00" + scripts[name] + "\x00")
	}
	identity := sha256.Sum256([]byte(content.String()))
	proxy := filepath.Join(extension, "v2-"+hex.EncodeToString(identity[:]))
	if err := os.MkdirAll(proxy, 0o700); err != nil {
		return nil, err
	}
	environment := []string{
		hookrelay.EnvPrefix + "CLIENT=" + helper, hookrelay.EnvPrefix + "ORIGINAL_DIR=" + directory,
		hookrelay.EnvPrefix + "URL=" + setting.LocalURL, hookrelay.EnvPrefix + "TOKEN=" + setting.InternalToken,
		hookrelay.EnvPrefix + "GIT=" + gitcmd.GitExecutable,
		hookrelay.EnvPrefix + "VERBOSE=" + strconv.FormatBool(setting.Git.VerbosePush),
		hookrelay.EnvPrefix + "VERBOSE_DELAY=" + setting.Git.VerbosePushDelay.String(),
		hookrelay.EnvPrefix + "PROC_RECEIVE=" + strconv.FormatBool(git.DefaultFeatures().SupportProcReceive),
	}
	for index, name := range names {
		if index < len(delegates) && name != "proc-receive" {
			digest := sha256.Sum256([]byte(delegates[index]))
			environment = append(environment, hookrelay.EnvPrefix+"SHA_"+strings.ToUpper(strings.ReplaceAll(name, "-", "_"))+"="+hex.EncodeToString(digest[:]))
			dispatchDigest := sha256.Sum256([]byte(templates[index]))
			environment = append(environment, hookrelay.EnvPrefix+"DISPATCH_SHA_"+strings.ToUpper(strings.ReplaceAll(name, "-", "_"))+"="+hex.EncodeToString(dispatchDigest[:]))
			if direct {
				if err := ensureRelayLink(proxy, name, helper); err != nil {
					return nil, err
				}
				continue
			}
		}
		if err := ensureRelayFile(proxy, name, scripts[name]); err != nil {
			return nil, err
		}
	}
	command.AddConfig("core.hooksPath", proxy)
	return environment, nil
}
