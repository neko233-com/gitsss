// Copyright 2026 gitsss Authors. All rights reserved.

package autostart

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrInvalidTransaction = errors.New("invalid autostart transaction")

type transactionHost interface {
	EnsureDirectory(string, fs.FileMode) (bool, error)
	CreateToken(context.Context, Platform, string, []byte) error
	CreateDefinition(string, []byte, fs.FileMode) error
	Run(context.Context, Command) error
	RemoveFile(string) error
	RemoveDirectory(string) error
}

type osTransactionHost struct{}

func Install(ctx context.Context, plan Plan, token string) error {
	return installWithHost(ctx, plan, token, osTransactionHost{})
}

func Uninstall(ctx context.Context, plan Plan) error {
	return uninstallWithHost(ctx, plan, osTransactionHost{})
}

func installWithHost(ctx context.Context, plan Plan, token string, host transactionHost) error {
	if err := validateTransaction(plan, true); err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 4095 || strings.ContainsAny(token, "\x00\r\n") {
		return fmt.Errorf("%w: token must be one non-empty line up to 4095 bytes", ErrInvalidTransaction)
	}
	createdDirectories := make([]string, 0, len(plan.Directories))
	createdFiles := make([]string, 0, 2)
	rollback := func(cause error, serviceAttempted bool) error {
		failures := []error{cause}
		if serviceAttempted {
			for _, command := range plan.Uninstall {
				if err := host.Run(ctx, command); err != nil {
					failures = append(failures, fmt.Errorf("rollback service: %w", err))
				}
			}
		}
		for index := len(createdFiles) - 1; index >= 0; index-- {
			if err := host.RemoveFile(createdFiles[index]); err != nil && !errors.Is(err, fs.ErrNotExist) {
				failures = append(failures, fmt.Errorf("rollback file %s: %w", createdFiles[index], err))
			}
		}
		for index := len(createdDirectories) - 1; index >= 0; index-- {
			_ = host.RemoveDirectory(createdDirectories[index])
		}
		return errors.Join(failures...)
	}
	for _, directory := range plan.Directories {
		created, err := host.EnsureDirectory(directory, 0o755)
		if err != nil {
			return rollback(fmt.Errorf("create service directory %s: %w", directory, err), false)
		}
		if created {
			createdDirectories = append(createdDirectories, directory)
		}
	}
	if err := host.CreateToken(ctx, plan.Platform, plan.TokenFile, []byte(token+"\n")); err != nil {
		return rollback(fmt.Errorf("create protected token: %w", err), false)
	}
	createdFiles = append(createdFiles, plan.TokenFile)
	if err := host.CreateDefinition(plan.DefinitionPath, []byte(plan.Definition), 0o644); err != nil {
		return rollback(fmt.Errorf("create service definition: %w", err), false)
	}
	createdFiles = append(createdFiles, plan.DefinitionPath)
	for _, command := range plan.Install {
		if err := host.Run(ctx, command); err != nil {
			return rollback(fmt.Errorf("install service: %w", err), true)
		}
	}
	return nil
}

func uninstallWithHost(ctx context.Context, plan Plan, host transactionHost) error {
	if err := validateTransaction(plan, false); err != nil {
		return err
	}
	var failures []error
	for _, command := range plan.Uninstall {
		if err := host.Run(ctx, command); err != nil {
			failures = append(failures, fmt.Errorf("uninstall service: %w", err))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	for _, filePath := range []string{plan.TokenFile, plan.DefinitionPath} {
		if err := host.RemoveFile(filePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failures = append(failures, fmt.Errorf("remove %s: %w", filePath, err))
		}
	}
	for index := len(plan.Directories) - 1; index >= 0; index-- {
		_ = host.RemoveDirectory(plan.Directories[index])
	}
	return errors.Join(failures...)
}

func validateTransaction(plan Plan, requireInstall bool) error {
	if plan.Platform != Platform(runtime.GOOS) {
		return fmt.Errorf("%w: target platform %s does not match host %s", ErrInvalidTransaction, plan.Platform, runtime.GOOS)
	}
	if !validTargetAbsolutePath(plan.Platform, plan.TokenFile) || !validTargetAbsolutePath(plan.Platform, plan.DefinitionPath) {
		return fmt.Errorf("%w: token and definition paths must be absolute", ErrInvalidTransaction)
	}
	for _, directory := range plan.Directories {
		if !validTargetAbsolutePath(plan.Platform, directory) {
			return fmt.Errorf("%w: service directories must be absolute", ErrInvalidTransaction)
		}
	}
	if requireInstall && (plan.Definition == "" || len(plan.Install) == 0) {
		return fmt.Errorf("%w: definition and install commands are required", ErrInvalidTransaction)
	}
	if len(plan.Uninstall) == 0 {
		return fmt.Errorf("%w: uninstall commands are required", ErrInvalidTransaction)
	}
	commands := append(append([]Command{}, plan.Install...), plan.Uninstall...)
	for _, command := range commands {
		if command.Executable == "" || strings.ContainsAny(command.Executable, "\x00\r\n") {
			return fmt.Errorf("%w: invalid service command", ErrInvalidTransaction)
		}
		for _, argument := range command.Arguments {
			if strings.ContainsAny(argument, "\x00\r\n") {
				return fmt.Errorf("%w: invalid service argument", ErrInvalidTransaction)
			}
		}
	}
	return nil
}

func (osTransactionHost) EnsureDirectory(directory string, mode fs.FileMode) (bool, error) {
	info, err := os.Stat(directory)
	if err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("path is not a directory")
		}
		return false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(directory, mode); err != nil {
		return false, err
	}
	return true, nil
}

func (osTransactionHost) CreateToken(ctx context.Context, platform Platform, filePath string, content []byte) error {
	return atomicCreateNoReplace(filePath, content, 0o600, func(temporaryPath string) error {
		if err := os.Chmod(temporaryPath, 0o600); err != nil {
			return err
		}
		if platform != PlatformWindows {
			return nil
		}
		return runOSCommand(ctx, Command{
			Executable: "icacls.exe",
			Arguments: []string{
				temporaryPath, "/inheritance:r", "/grant:r",
				"*S-1-5-18:(R)", "*S-1-5-32-544:(R)",
			},
		})
	})
}

func (osTransactionHost) CreateDefinition(filePath string, content []byte, mode fs.FileMode) error {
	return atomicCreateNoReplace(filePath, content, mode, nil)
}

func (osTransactionHost) Run(ctx context.Context, command Command) error {
	return runOSCommand(ctx, command)
}

func (osTransactionHost) RemoveFile(filePath string) error {
	return os.Remove(filePath)
}

func (osTransactionHost) RemoveDirectory(directory string) error {
	return os.Remove(directory)
}

func atomicCreateNoReplace(filePath string, content []byte, mode fs.FileMode, protect func(string) error) error {
	directory := filepath.Dir(filePath)
	temporary, err := os.CreateTemp(directory, ".gitsss-partial-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	completed := false
	linked := false
	defer func() {
		if !completed {
			temporary.Close()
			os.Remove(temporaryPath)
			if linked {
				os.Remove(filePath)
			}
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if protect != nil {
		if err := protect(temporaryPath); err != nil {
			return err
		}
	}
	if err := os.Link(temporaryPath, filePath); err != nil {
		return err
	}
	linked = true
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		completed = true
		return nil
	}
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer directoryHandle.Close()
	if err := directoryHandle.Sync(); err != nil {
		return err
	}
	completed = true
	return nil
}

func runOSCommand(ctx context.Context, command Command) error {
	process := exec.CommandContext(ctx, command.Executable, command.Arguments...)
	output, err := process.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", command.Executable, err)
	}
	return fmt.Errorf("%s: %w: %s", command.Executable, err, detail)
}
