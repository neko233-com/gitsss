// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/takeover"
	"modernc.org/sqlite"
)

const (
	databaseSnapshotKind  = "database-snapshot"
	sqliteSnapshotTimeout = 10 * time.Minute
)

type PreparedPlan struct {
	Plan
	temporaryDirectory string
}

func PreparePlan(report takeover.Report, now time.Time) (*PreparedPlan, error) {
	if report.Database.Type == "postgres" || report.Database.Type == "mysql" {
		return prepareNativeDatabasePlan(report, now)
	}
	if report.Database.Type != "sqlite3" {
		plan, err := BuildPlan(report, now)
		if err != nil {
			return nil, err
		}
		return &PreparedPlan{Plan: plan}, nil
	}
	database, found := shadowAsset(report.Assets, "database")
	if !found || !database.Exists || !database.Readable || database.Truncated || database.Files != 1 {
		return nil, fmt.Errorf("%w: SQLite database was not fully assessed", ErrInvalidBackup)
	}
	temporaryDirectory, err := os.MkdirTemp("", "gitsss-sqlite-snapshot-")
	if err != nil {
		return nil, err
	}
	prepared := &PreparedPlan{temporaryDirectory: temporaryDirectory}
	cleanup := func() {
		_ = prepared.Close()
	}
	snapshotPath := filepath.Join(temporaryDirectory, "gitea.db")
	if err := snapshotSQLite(database.Path, snapshotPath); err != nil {
		cleanup()
		return nil, err
	}
	info, err := os.Stat(snapshotPath)
	if err != nil {
		cleanup()
		return nil, err
	}
	snapshotReport := report
	snapshotReport.Assets = append([]takeover.Asset(nil), report.Assets...)
	for index := range snapshotReport.Assets {
		if snapshotReport.Assets[index].Kind != "database" {
			continue
		}
		snapshotReport.Assets[index] = takeover.Asset{
			Kind: databaseSnapshotKind, Path: snapshotPath, Exists: true, Readable: true,
			Files: 1, Bytes: info.Size(),
		}
	}
	prepared.Plan, err = BuildPlan(snapshotReport, now)
	if err != nil {
		cleanup()
		return nil, err
	}
	return prepared, nil
}

func (prepared *PreparedPlan) Close() error {
	if prepared == nil || prepared.temporaryDirectory == "" {
		return nil
	}
	directory := prepared.temporaryDirectory
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	prepared.temporaryDirectory = ""
	return nil
}

func ExportPrepared(outputPath, passphrase string, prepared *PreparedPlan) (err error) {
	if prepared == nil {
		return fmt.Errorf("%w: prepared plan is required", ErrInvalidBackup)
	}
	defer func() {
		err = errors.Join(err, prepared.Close())
	}()
	return Export(outputPath, passphrase, prepared.Plan)
}

func snapshotSQLite(sourcePath, destinationPath string) error {
	source, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	destination, err := filepath.Abs(destinationPath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: SQLite source is not a regular file", ErrInvalidBackup)
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("%w: SQLite snapshot already exists", ErrInvalidBackup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), sqliteSnapshotTimeout)
	defer cancel()
	database, err := sql.Open("sqlite", sqliteReadOnlyDSN(source))
	if err != nil {
		return err
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	connection, err := database.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	err = connection.Raw(func(driverConnection any) error {
		backuper, ok := driverConnection.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return errors.New("SQLite driver does not support online backup")
		}
		backup, err := backuper.NewBackup(destination)
		if err != nil {
			return err
		}
		finished := false
		defer func() {
			if !finished {
				_ = backup.Finish()
			}
		}()
		for more := true; more; {
			if err := ctx.Err(); err != nil {
				return err
			}
			more, err = backup.Step(256)
			if err != nil {
				return err
			}
		}
		err = backup.Finish()
		finished = true
		return err
	})
	if err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("create SQLite online snapshot: %w", err)
	}
	if err := os.Chmod(destination, 0o600); err != nil {
		_ = os.Remove(destination)
		return err
	}
	file, err := os.OpenFile(destination, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := validateSQLiteSnapshot(destination); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return nil
}

func validateSQLiteSnapshot(snapshotPath string) error {
	database, err := sql.Open("sqlite", sqliteReadOnlyDSN(snapshotPath))
	if err != nil {
		return err
	}
	defer database.Close()
	var result string
	if err := database.QueryRow("PRAGMA quick_check").Scan(&result); err != nil {
		return fmt.Errorf("validate SQLite snapshot: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("%w: SQLite quick_check returned %q", ErrIntegrity, result)
	}
	return nil
}

func sqliteReadOnlyDSN(filePath string) string {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	query := make(url.Values)
	query.Set("mode", "ro")
	query.Add("_pragma", "busy_timeout(5000)")
	return "file:" + strings.Join(parts, "/") + "?" + query.Encode()
}
