// Copyright 2026 gitsss Authors. All rights reserved.

package migration

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	maxPortableLine   = 16 << 20
	maxInventoryRows  = 100_000_000
	maxInventoryTable = 10_000
)

var ErrInvalidSnapshot = errors.New("invalid source database snapshot")

type Counts struct {
	Users         int64 `json:"users"`
	Organizations int64 `json:"organizations"`
	Repositories  int64 `json:"repositories"`
	Issues        int64 `json:"issues"`
	PullRequests  int64 `json:"pull_requests"`
	Comments      int64 `json:"comments"`
	Releases      int64 `json:"releases"`
	Labels        int64 `json:"labels"`
	Milestones    int64 `json:"milestones"`
	Attachments   int64 `json:"attachments"`
	Teams         int64 `json:"teams"`
	ActionRuns    int64 `json:"action_runs"`
}

type Inventory struct {
	Database    string           `json:"database"`
	Snapshot    string           `json:"snapshot"`
	Tables      map[string]int64 `json:"tables"`
	Counts      Counts           `json:"counts"`
	Digest      string           `json:"digest"`
	InspectedAt time.Time        `json:"inspected_at"`
}

type portableHeader struct {
	Format   string `json:"format"`
	Database string `json:"database"`
}

type portableRecord struct {
	Table   string          `json:"table"`
	Columns []string        `json:"columns"`
	Types   []string        `json:"types"`
	Values  []portableValue `json:"values"`
}

type portableValue struct {
	Kind string `json:"kind"`
	Data string `json:"data,omitempty"`
}

func Inspect(restoredContent string, now time.Time) (Inventory, error) {
	root, err := filepath.Abs(restoredContent)
	if err != nil {
		return Inventory{}, err
	}
	directory := filepath.Join(root, "data", "database-snapshot")
	info, err := os.Lstat(directory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Inventory{}, fmt.Errorf("%w: database-snapshot directory is required", ErrInvalidSnapshot)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return Inventory{}, err
	}
	files := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(directory, entry.Name()))
	}
	if len(files) != 1 {
		return Inventory{}, fmt.Errorf("%w: exactly one database snapshot is required", ErrInvalidSnapshot)
	}
	if now.IsZero() {
		now = time.Now()
	}
	inventory, err := inspectSnapshot(files[0])
	if err != nil {
		return Inventory{}, err
	}
	inventory.Snapshot = files[0]
	inventory.InspectedAt = now.UTC()
	inventory.Digest, err = inventoryDigest(inventory)
	if err != nil {
		return Inventory{}, err
	}
	return inventory, nil
}

func inventoryDigest(inventory Inventory) (string, error) {
	payload, err := json.Marshal(struct {
		Database string           `json:"database"`
		Tables   map[string]int64 `json:"tables"`
		Counts   Counts           `json:"counts"`
	}{Database: inventory.Database, Tables: inventory.Tables, Counts: inventory.Counts})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func inspectSnapshot(snapshot string) (Inventory, error) {
	file, err := os.Open(snapshot)
	if err != nil {
		return Inventory{}, err
	}
	var prefix [16]byte
	count, readErr := file.Read(prefix[:])
	closeErr := file.Close()
	if readErr != nil && count == 0 {
		return Inventory{}, readErr
	}
	if closeErr != nil {
		return Inventory{}, closeErr
	}
	if count >= 16 && string(prefix[:16]) == "SQLite format 3\x00" {
		return inspectSQLite(snapshot)
	}
	return inspectPortable(snapshot)
}

func inspectSQLite(snapshot string) (Inventory, error) {
	database, err := sql.Open("sqlite", sqliteReadOnlyDSN(snapshot))
	if err != nil {
		return Inventory{}, err
	}
	defer database.Close()
	var integrity string
	if err := database.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil || integrity != "ok" {
		return Inventory{}, fmt.Errorf("%w: SQLite quick_check failed", ErrInvalidSnapshot)
	}
	rows, err := database.Query("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return Inventory{}, err
	}
	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return Inventory{}, err
		}
		tables = append(tables, table)
		if len(tables) > maxInventoryTable {
			rows.Close()
			return Inventory{}, fmt.Errorf("%w: too many tables", ErrInvalidSnapshot)
		}
	}
	if err := rows.Close(); err != nil {
		return Inventory{}, err
	}
	inventory := Inventory{Database: "sqlite3", Tables: make(map[string]int64, len(tables))}
	var totalRows int64
	for _, table := range tables {
		var rowCount int64
		if err := database.QueryRow("SELECT COUNT(*) FROM " + quoteSQLite(table)).Scan(&rowCount); err != nil {
			return Inventory{}, err
		}
		if rowCount < 0 || rowCount > maxInventoryRows {
			return Inventory{}, fmt.Errorf("%w: table %s exceeds row limit", ErrInvalidSnapshot, table)
		}
		totalRows += rowCount
		if totalRows > maxInventoryRows {
			return Inventory{}, fmt.Errorf("%w: snapshot exceeds row limit", ErrInvalidSnapshot)
		}
		inventory.Tables[table] = rowCount
	}
	inventory.Counts = countsFromTables(inventory.Tables)
	if _, exists := inventory.Tables["user"]; exists {
		if exists, err := sqliteColumnExists(database, "user", "type"); err != nil {
			return Inventory{}, err
		} else if exists {
			users, organizations, err := sqliteUserCounts(database)
			if err != nil {
				return Inventory{}, err
			}
			inventory.Counts.Users = users
			inventory.Counts.Organizations = organizations
		}
	}
	if _, exists := inventory.Tables["issue"]; exists {
		if exists, err := sqliteColumnExists(database, "issue", "is_pull"); err != nil {
			return Inventory{}, err
		} else if exists {
			issues, pulls, err := sqliteIssueCounts(database)
			if err != nil {
				return Inventory{}, err
			}
			inventory.Counts.Issues = issues
			if pulls > inventory.Counts.PullRequests {
				inventory.Counts.PullRequests = pulls
			}
		}
	}
	return inventory, nil
}

func inspectPortable(snapshot string) (Inventory, error) {
	file, err := os.Open(snapshot)
	if err != nil {
		return Inventory{}, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), maxPortableLine)
	if !scanner.Scan() {
		return Inventory{}, fmt.Errorf("%w: missing portable header", ErrInvalidSnapshot)
	}
	var header portableHeader
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil || header.Format != "gitsss-database-v1" || (header.Database != "postgres" && header.Database != "mysql") {
		return Inventory{}, fmt.Errorf("%w: unsupported portable header", ErrInvalidSnapshot)
	}
	inventory := Inventory{Database: header.Database, Tables: map[string]int64{}}
	columns := map[string][]string{}
	var totalRows int64
	for scanner.Scan() {
		var record portableRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil || strings.TrimSpace(record.Table) == "" {
			return Inventory{}, fmt.Errorf("%w: malformed portable record", ErrInvalidSnapshot)
		}
		table := normalizedTable(record.Table)
		if len(record.Columns) > 0 {
			if len(record.Columns) != len(record.Types) || len(record.Columns) > 4_096 {
				return Inventory{}, fmt.Errorf("%w: invalid table declaration", ErrInvalidSnapshot)
			}
			if _, exists := columns[record.Table]; exists || len(columns) >= maxInventoryTable {
				return Inventory{}, fmt.Errorf("%w: duplicate or excessive table declaration", ErrInvalidSnapshot)
			}
			columns[record.Table] = append([]string(nil), record.Columns...)
			if _, exists := inventory.Tables[table]; !exists {
				inventory.Tables[table] = 0
			}
			continue
		}
		declared, exists := columns[record.Table]
		if !exists || len(record.Values) != len(declared) {
			return Inventory{}, fmt.Errorf("%w: row does not match table declaration", ErrInvalidSnapshot)
		}
		totalRows++
		if totalRows > maxInventoryRows {
			return Inventory{}, fmt.Errorf("%w: snapshot exceeds row limit", ErrInvalidSnapshot)
		}
		inventory.Tables[table]++
		classifyPortableRow(&inventory.Counts, table, declared, record.Values)
	}
	if err := scanner.Err(); err != nil {
		return Inventory{}, fmt.Errorf("%w: %v", ErrInvalidSnapshot, err)
	}
	base := countsFromTables(inventory.Tables)
	mergeUnclassifiedCounts(&inventory.Counts, base)
	return inventory, nil
}

func countsFromTables(tables map[string]int64) Counts {
	return Counts{
		Users: tables["user"], Repositories: tables["repository"], Issues: tables["issue"],
		PullRequests: tables["pull_request"], Comments: tables["comment"], Releases: tables["release"],
		Labels: tables["label"], Milestones: tables["milestone"], Attachments: tables["attachment"],
		Teams: tables["team"], ActionRuns: tables["action_run"],
	}
}

func mergeUnclassifiedCounts(counts *Counts, base Counts) {
	if counts.Users == 0 && counts.Organizations == 0 {
		counts.Users = base.Users
	}
	counts.Repositories = base.Repositories
	if counts.Issues == 0 && counts.PullRequests == 0 {
		counts.Issues = base.Issues
	}
	if base.PullRequests > counts.PullRequests {
		counts.PullRequests = base.PullRequests
	}
	counts.Comments = base.Comments
	counts.Releases = base.Releases
	counts.Labels = base.Labels
	counts.Milestones = base.Milestones
	counts.Attachments = base.Attachments
	counts.Teams = base.Teams
	counts.ActionRuns = base.ActionRuns
}

func classifyPortableRow(counts *Counts, table string, columns []string, values []portableValue) {
	switch table {
	case "user":
		if portableInteger(columns, values, "type") == 1 {
			counts.Organizations++
		} else {
			counts.Users++
		}
	case "issue":
		if portableBoolean(columns, values, "is_pull") {
			counts.PullRequests++
		} else {
			counts.Issues++
		}
	}
}

func portableInteger(columns []string, values []portableValue, name string) int64 {
	for index, column := range columns {
		if column != name || index >= len(values) {
			continue
		}
		value := values[index]
		if value.Kind == "int" || value.Kind == "uint" {
			parsed, _ := strconv.ParseInt(value.Data, 10, 64)
			return parsed
		}
		if value.Kind == "bytes" || value.Kind == "string" {
			decoded, _ := base64.StdEncoding.DecodeString(value.Data)
			parsed, _ := strconv.ParseInt(string(decoded), 10, 64)
			return parsed
		}
	}
	return 0
}

func portableBoolean(columns []string, values []portableValue, name string) bool {
	for index, column := range columns {
		if column != name || index >= len(values) {
			continue
		}
		value := values[index]
		if value.Kind == "bool" {
			return value.Data == "true"
		}
		return portableInteger(columns[index:index+1], values[index:index+1], name) != 0
	}
	return false
}

func sqliteUserCounts(database *sql.DB) (int64, int64, error) {
	var users, organizations int64
	if err := database.QueryRow("SELECT COUNT(*) FROM \"user\" WHERE type = 0 OR type IS NULL").Scan(&users); err != nil {
		return 0, 0, err
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM \"user\" WHERE type = 1").Scan(&organizations); err != nil {
		return 0, 0, err
	}
	return users, organizations, nil
}

func sqliteIssueCounts(database *sql.DB) (int64, int64, error) {
	var issues, pulls int64
	if err := database.QueryRow("SELECT COUNT(*) FROM \"issue\" WHERE is_pull = 0 OR is_pull IS NULL").Scan(&issues); err != nil {
		return 0, 0, err
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM \"issue\" WHERE is_pull = 1").Scan(&pulls); err != nil {
		return 0, 0, err
	}
	return issues, pulls, nil
}

func sqliteColumnExists(database *sql.DB, table, column string) (bool, error) {
	rows, err := database.Query("PRAGMA table_info(" + quoteSQLite(table) + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var sequence int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&sequence, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func quoteSQLite(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func normalizedTable(table string) string {
	if index := strings.LastIndex(table, "."); index >= 0 {
		return table[index+1:]
	}
	return table
}

func sqliteReadOnlyDSN(filePath string) string {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return "file:" + strings.Join(parts, "/") + "?mode=ro"
}
