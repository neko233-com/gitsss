// Copyright 2026 gitsss Authors. All rights reserved.

package migration

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/state"
	"github.com/neko233-com/gitsss/server/internal/storage"
)

const (
	materializationVersion = 1
	importBatchSize        = 250
	maxImportedPayload     = 900 << 10
	maxMaterializedFiles   = 10_000_000
)

var ErrInvalidMaterialization = errors.New("invalid gitsss materialization")

var importedTables = map[string]struct{}{
	"user": {}, "repository": {}, "issue": {}, "pull_request": {}, "comment": {},
	"release": {}, "label": {}, "milestone": {}, "attachment": {}, "team": {},
	"action_run": {}, "org_user": {}, "team_user": {}, "team_repo": {},
	"issue_label": {}, "issue_assignees": {}, "issue_user": {}, "release_asset": {},
	"access": {}, "collaboration": {}, "deploy_key": {}, "public_key": {},
	"protected_branch": {}, "protected_tag": {}, "webhook": {}, "watch": {},
	"star": {}, "follow": {}, "topic": {}, "repo_topic": {}, "repo_unit": {},
	"repo_redirect": {}, "branch": {}, "email_address": {}, "login_source": {},
	"user_open_id": {}, "user_redirect": {}, "oauth2_application": {},
	"issue_dependency": {}, "issue_watch": {}, "review": {}, "review_state": {},
	"tracked_time": {}, "reaction": {}, "project": {}, "project_board": {},
	"project_issue": {}, "action_runner": {}, "action_schedule": {},
	"action_task": {}, "action_task_output": {}, "action_artifact": {},
}

type Materialization struct {
	Version         int              `json:"version"`
	Target          string           `json:"target"`
	ContentPath     string           `json:"content_path"`
	ActivePath      string           `json:"active_path"`
	InventorySHA256 string           `json:"inventory_sha256"`
	ContentSHA256   string           `json:"content_sha256"`
	SourceSHA256    string           `json:"source_sha256"`
	Counts          Counts           `json:"counts"`
	ImportedRows    map[string]int64 `json:"imported_rows"`
	Repositories    int              `json:"repositories"`
	MaterializedAt  time.Time        `json:"materialized_at"`
}

type importedRow struct {
	SourceTable string         `json:"source_table"`
	Ordinal     int64          `json:"ordinal"`
	Values      map[string]any `json:"values,omitempty"`
	External    string         `json:"external,omitempty"`
	SHA256      string         `json:"sha256,omitempty"`
}

type repositoryCatalogRecord struct {
	ID            string    `json:"id"`
	SourceID      string    `json:"source_id,omitempty"`
	Name          string    `json:"name"`
	DefaultBranch string    `json:"default_branch"`
	Path          string    `json:"path"`
	Hosted        bool      `json:"hosted"`
	CreatedAt     time.Time `json:"created_at"`
}

func Materialize(restoredContent, outputTarget, activePath, expectedInventory string, now time.Time) (Materialization, error) {
	if len(expectedInventory) != sha256.Size*2 || strings.ToLower(expectedInventory) != expectedInventory {
		return Materialization{}, fmt.Errorf("%w: expected inventory SHA-256 is required", ErrInvalidMaterialization)
	}
	if _, err := hex.DecodeString(expectedInventory); err != nil {
		return Materialization{}, fmt.Errorf("%w: invalid inventory SHA-256", ErrInvalidMaterialization)
	}
	source, target, active, err := materializationPaths(restoredContent, outputTarget, activePath)
	if err != nil {
		return Materialization{}, err
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		return Materialization{}, fmt.Errorf("%w: output target must not exist", ErrInvalidMaterialization)
	}
	inventory, err := Inspect(source, now)
	if err != nil {
		return Materialization{}, err
	}
	if inventory.Digest != expectedInventory {
		return Materialization{}, fmt.Errorf("%w: business inventory changed", ErrInvalidMaterialization)
	}
	if err := admitMaterialization(source, filepath.Dir(target), inventory.Snapshot); err != nil {
		return Materialization{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	temporary, err := os.MkdirTemp(filepath.Dir(target), ".gitsss-materialize-")
	if err != nil {
		return Materialization{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(temporary)
		}
	}()
	content := filepath.Join(temporary, "content")
	if err := os.MkdirAll(content, 0o700); err != nil {
		return Materialization{}, err
	}
	store, err := state.Open(filepath.Join(content, "gitsss.db"))
	if err != nil {
		return Materialization{}, err
	}
	imported, importErr := importBusinessRows(store, inventory.Snapshot, filepath.Join(content, "imports", "oversize"))
	closeErr := store.Close()
	if importErr != nil {
		return Materialization{}, importErr
	}
	if closeErr != nil {
		return Materialization{}, closeErr
	}
	for table, rows := range imported {
		if expected, exists := inventory.Tables[table]; exists && expected != rows {
			return Materialization{}, fmt.Errorf("%w: imported %s rows changed from %d to %d", ErrInvalidMaterialization, table, expected, rows)
		}
	}
	if err := copyFile(inventory.Snapshot, filepath.Join(content, "imports", "source-database", filepath.Base(inventory.Snapshot))); err != nil {
		return Materialization{}, err
	}
	sourceDigest, err := fileSHA256(inventory.Snapshot)
	if err != nil {
		return Materialization{}, err
	}
	repositoryCount, err := materializeRepositories(source, content, active, now)
	if err != nil {
		return Materialization{}, err
	}
	if int64(repositoryCount) != inventory.Counts.Repositories {
		return Materialization{}, fmt.Errorf("%w: database lists %d repositories but %d bare repositories were found", ErrInvalidMaterialization, inventory.Counts.Repositories, repositoryCount)
	}
	for _, asset := range []struct{ source, target string }{
		{source: filepath.Join(source, "data", "lfs"), target: filepath.Join(content, "lfs")},
		{source: filepath.Join(source, "data", "attachments"), target: filepath.Join(content, "imports", "attachments")},
		{source: filepath.Join(source, "data", "packages"), target: filepath.Join(content, "imports", "packages")},
		{source: filepath.Join(source, "data", "actions-log"), target: filepath.Join(content, "imports", "actions-log")},
		{source: filepath.Join(source, "data", "actions-artifacts"), target: filepath.Join(content, "imports", "actions-artifacts")},
	} {
		if err := copyOptionalTree(asset.source, asset.target); err != nil {
			return Materialization{}, err
		}
	}
	contentDigest, err := materializedTreeDigest(content)
	if err != nil {
		return Materialization{}, err
	}
	result := Materialization{
		Version: materializationVersion, Target: target, ContentPath: filepath.Join(target, "content"),
		ActivePath: active, InventorySHA256: inventory.Digest, ContentSHA256: contentDigest,
		SourceSHA256: sourceDigest, Counts: inventory.Counts, ImportedRows: imported,
		Repositories: repositoryCount, MaterializedAt: now.UTC(),
	}
	if err := writeMaterializationManifest(content, result); err != nil {
		return Materialization{}, err
	}
	if err := os.Rename(temporary, target); err != nil {
		return Materialization{}, err
	}
	cleanup = false
	return result, nil
}

func VerifyMaterialization(target, expectedInventory, expectedContent string) (Materialization, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(target))
	if err != nil {
		return Materialization{}, err
	}
	content := filepath.Join(absolute, "content")
	payload, err := os.ReadFile(filepath.Join(content, "gitsss-materialization.json"))
	if err != nil || len(payload) > 256<<10 {
		return Materialization{}, fmt.Errorf("%w: manifest is unavailable", ErrInvalidMaterialization)
	}
	var manifest Materialization
	if err := json.Unmarshal(payload, &manifest); err != nil || manifest.Version != materializationVersion || manifest.Target != absolute || manifest.ContentPath != content || manifest.InventorySHA256 != expectedInventory || manifest.ContentSHA256 != expectedContent {
		return Materialization{}, fmt.Errorf("%w: manifest does not match the request", ErrInvalidMaterialization)
	}
	digest, err := materializedTreeDigest(content)
	if err != nil {
		return Materialization{}, err
	}
	if digest != manifest.ContentSHA256 {
		return Materialization{}, fmt.Errorf("%w: content digest changed", ErrInvalidMaterialization)
	}
	return manifest, nil
}

func materializationPaths(restoredContent, outputTarget, activePath string) (string, string, string, error) {
	source, err := filepath.Abs(strings.TrimSpace(restoredContent))
	if err != nil {
		return "", "", "", err
	}
	target, err := filepath.Abs(strings.TrimSpace(outputTarget))
	if err != nil {
		return "", "", "", err
	}
	active, err := filepath.Abs(strings.TrimSpace(activePath))
	if err != nil {
		return "", "", "", err
	}
	for _, path := range []string{source, filepath.Dir(target), filepath.Dir(active)} {
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", "", "", fmt.Errorf("%w: paths require existing plain parent directories", ErrInvalidMaterialization)
		}
	}
	if source == target || source == active || target == active || containsPath(source, target) || containsPath(target, source) || containsPath(active, target) || containsPath(target, active) || filepath.Dir(active) == active {
		return "", "", "", fmt.Errorf("%w: source, output and active paths must not overlap", ErrInvalidMaterialization)
	}
	return filepath.Clean(source), filepath.Clean(target), filepath.Clean(active), nil
}

func containsPath(parent, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func admitMaterialization(source, targetParent, snapshot string) error {
	var projected int64
	entries := 0
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > maxMaterializedFiles {
			return fmt.Errorf("%w: source exceeds file limit", ErrInvalidMaterialization)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: source contains a symbolic link", ErrInvalidMaterialization)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%w: source contains a special file", ErrInvalidMaterialization)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 0 && projected > int64(^uint64(0)>>1)-info.Size() {
			return fmt.Errorf("%w: projected size overflow", ErrInvalidMaterialization)
		}
		projected += info.Size()
		return nil
	})
	if err != nil {
		return err
	}
	snapshotInfo, err := os.Stat(snapshot)
	if err != nil {
		return err
	}
	if snapshotInfo.Size() > 0 && projected > int64(^uint64(0)>>1)-snapshotInfo.Size() {
		return fmt.Errorf("%w: projected size overflow", ErrInvalidMaterialization)
	}
	projected += snapshotInfo.Size()
	service, err := storage.NewLiveService(storage.LiveOptions{VolumePath: targetParent, ReserveBytes: 10 << 30})
	if err != nil {
		return fmt.Errorf("%w: cannot measure output capacity: %v", ErrInvalidMaterialization, err)
	}
	admission := service.Admit(projected)
	if !admission.Allowed {
		return fmt.Errorf("%w: output capacity rejected %d projected bytes: %s", ErrInvalidMaterialization, projected, admission.Reason)
	}
	return nil
}

func importBusinessRows(store *state.Store, snapshot, overflowRoot string) (map[string]int64, error) {
	counts := map[string]int64{}
	batches := map[string][]state.Document{}
	flush := func(namespace string) error {
		if err := store.PutMany(namespace, batches[namespace]); err != nil {
			return err
		}
		batches[namespace] = batches[namespace][:0]
		return nil
	}
	err := visitSnapshotRows(snapshot, func(table string, values map[string]any) error {
		if _, imported := importedTables[table]; !imported {
			return nil
		}
		counts[table]++
		ordinal := counts[table]
		record := importedRow{SourceTable: table, Ordinal: ordinal, Values: values}
		payload, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if len(payload) > maxImportedPayload {
			relative := filepath.Join(table, fmt.Sprintf("%020d.json", ordinal))
			path := filepath.Join(overflowRoot, relative)
			if err := writeExclusiveFile(path, payload); err != nil {
				return err
			}
			digest := sha256.Sum256(payload)
			payload, err = json.Marshal(importedRow{SourceTable: table, Ordinal: ordinal, External: filepath.ToSlash(relative), SHA256: hex.EncodeToString(digest[:])})
			if err != nil {
				return err
			}
		}
		namespace := "source-import-" + table
		batches[namespace] = append(batches[namespace], state.Document{ID: fmt.Sprintf("%020d", ordinal), Payload: payload})
		if len(batches[namespace]) >= importBatchSize {
			if err := flush(namespace); err != nil {
				return err
			}
		}
		if table == "issue" {
			repositoryID, err := sortableSourceID(importedString(values["repo_id"]))
			if err != nil {
				return err
			}
			indexPayload, err := indexedWorkItemPayload(ordinal, values)
			if err != nil {
				return err
			}
			const indexNamespace = "import-work-items"
			indexID := repositoryID + "-" + fmt.Sprintf("%020d", ordinal)
			batches[indexNamespace] = append(batches[indexNamespace], state.Document{ID: indexID, Payload: indexPayload})
			if len(batches[indexNamespace]) >= importBatchSize {
				return flush(indexNamespace)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for namespace, documents := range batches {
		if len(documents) > 0 {
			if err := flush(namespace); err != nil {
				return nil, err
			}
		}
	}
	return counts, nil
}

func sortableSourceID(value string) (string, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return "", fmt.Errorf("%w: imported repository ID is invalid", ErrInvalidMaterialization)
	}
	return fmt.Sprintf("%020d", parsed), nil
}

func indexedWorkItemPayload(ordinal int64, values map[string]any) ([]byte, error) {
	indexed := make(map[string]any, len(values))
	for key, value := range values {
		if (key == "content" || key == "body") && len(importedString(value)) > 64<<10 {
			indexed[key] = importedString(value)[:64<<10]
			indexed["_body_truncated"] = true
			continue
		}
		indexed[key] = value
	}
	return json.Marshal(importedRow{SourceTable: "issue", Ordinal: ordinal, Values: indexed})
}

func visitSnapshotRows(snapshot string, visit func(string, map[string]any) error) error {
	file, err := os.Open(snapshot)
	if err != nil {
		return err
	}
	var prefix [16]byte
	count, readErr := file.Read(prefix[:])
	if closeErr := file.Close(); readErr != nil || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if count == len(prefix) && string(prefix[:]) == "SQLite format 3\x00" {
		return visitSQLiteRows(snapshot, visit)
	}
	return visitPortableRows(snapshot, visit)
}

func visitSQLiteRows(snapshot string, visit func(string, map[string]any) error) error {
	database, err := sql.Open("sqlite", sqliteReadOnlyDSN(snapshot))
	if err != nil {
		return err
	}
	defer database.Close()
	tables := make([]string, 0, len(importedTables))
	for table := range importedTables {
		if exists, err := sqliteTableExists(database, table); err != nil {
			return err
		} else if exists {
			tables = append(tables, table)
		}
	}
	sort.Strings(tables)
	for _, table := range tables {
		order := ""
		if exists, err := sqliteColumnExists(database, table, "id"); err != nil {
			return err
		} else if exists {
			order = " ORDER BY \"id\""
		}
		rows, err := database.Query("SELECT * FROM " + quoteSQLite(table) + order)
		if err != nil {
			return err
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return err
		}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for index := range values {
				pointers[index] = &values[index]
			}
			if err := rows.Scan(pointers...); err != nil {
				rows.Close()
				return err
			}
			record := make(map[string]any, len(columns))
			for index, column := range columns {
				if bytes, ok := values[index].([]byte); ok {
					record[column] = append([]byte(nil), bytes...)
				} else {
					record[column] = values[index]
				}
			}
			if err := visit(table, record); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func sqliteTableExists(database *sql.DB, table string) (bool, error) {
	var count int
	err := database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count)
	return count == 1, err
}

func visitPortableRows(snapshot string, visit func(string, map[string]any) error) error {
	file, err := os.Open(snapshot)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), maxPortableLine)
	if !scanner.Scan() {
		return ErrInvalidSnapshot
	}
	var header portableHeader
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil || header.Format != "gitsss-database-v1" {
		return ErrInvalidSnapshot
	}
	columns := map[string][]string{}
	for scanner.Scan() {
		var record portableRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return ErrInvalidSnapshot
		}
		if len(record.Columns) > 0 {
			columns[record.Table] = append([]string(nil), record.Columns...)
			continue
		}
		table := normalizedTable(record.Table)
		if _, imported := importedTables[table]; !imported {
			continue
		}
		declared, exists := columns[record.Table]
		if !exists || len(declared) != len(record.Values) {
			return ErrInvalidSnapshot
		}
		values := make(map[string]any, len(declared))
		for index, column := range declared {
			decoded, err := decodePortableValue(record.Values[index])
			if err != nil {
				return err
			}
			values[column] = decoded
		}
		if err := visit(table, values); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func decodePortableValue(value portableValue) (any, error) {
	switch value.Kind {
	case "null":
		return nil, nil
	case "bytes":
		return base64.StdEncoding.DecodeString(value.Data)
	case "string":
		decoded, err := base64.StdEncoding.DecodeString(value.Data)
		return string(decoded), err
	case "bool":
		return strconv.ParseBool(value.Data)
	case "int":
		return strconv.ParseInt(value.Data, 10, 64)
	case "uint":
		return strconv.ParseUint(value.Data, 10, 64)
	case "float":
		return strconv.ParseFloat(value.Data, 64)
	case "time":
		if _, err := time.Parse(time.RFC3339Nano, value.Data); err != nil {
			return nil, err
		}
		return value.Data, nil
	default:
		return nil, fmt.Errorf("%w: unsupported portable value %q", ErrInvalidSnapshot, value.Kind)
	}
}

func materializeRepositories(source, content, active string, now time.Time) (int, error) {
	sourceRoot := filepath.Join(source, "data", "repositories")
	info, err := os.Lstat(sourceRoot)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return 0, fmt.Errorf("%w: repository source must be a plain directory", ErrInvalidMaterialization)
	}
	repositories := make([]string, 0)
	err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: repository tree contains a symbolic link", ErrInvalidMaterialization)
		}
		if path != sourceRoot && entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".git") {
			relative, err := filepath.Rel(sourceRoot, path)
			if err != nil {
				return err
			}
			repositories = append(repositories, filepath.ToSlash(relative))
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	sort.Strings(repositories)
	if err := os.MkdirAll(filepath.Join(content, "repositories"), 0o700); err != nil {
		return 0, err
	}
	store, err := state.Open(filepath.Join(content, "gitsss.db"))
	if err != nil {
		return 0, err
	}
	defer store.Close()
	sourceIDs, err := sourceRepositoryIDs(store)
	if err != nil {
		return 0, err
	}
	seen := map[string]struct{}{}
	count := 0
	for _, relative := range repositories {
		if strings.HasSuffix(strings.ToLower(relative), ".wiki.git") {
			if err := copyTree(filepath.Join(sourceRoot, filepath.FromSlash(relative)), filepath.Join(content, "imports", "wikis", filepath.FromSlash(relative))); err != nil {
				return 0, err
			}
			continue
		}
		id := repositoryID(relative, seen)
		sourceID := sourceIDs[strings.ToLower(strings.TrimSuffix(filepath.ToSlash(relative), ".git"))]
		if sourceID == "" {
			return 0, fmt.Errorf("%w: repository %s has no matching database record", ErrInvalidMaterialization, relative)
		}
		seen[id] = struct{}{}
		destination := filepath.Join(content, "repositories", id+".git")
		if err := copyTree(filepath.Join(sourceRoot, filepath.FromSlash(relative)), destination); err != nil {
			return 0, err
		}
		record := repositoryCatalogRecord{
			ID: id, SourceID: sourceID, Name: strings.TrimSuffix(relative, ".git"), DefaultBranch: repositoryDefaultBranch(destination),
			Path: filepath.Join(active, "repositories", id+".git"), Hosted: true, CreatedAt: now.UTC(),
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return 0, err
		}
		if err := store.Put("repository-catalog", id, payload); err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func sourceRepositoryIDs(store *state.Store) (map[string]string, error) {
	users := map[string]string{}
	if err := visitImportedDocuments(store, "user", func(_ string, values map[string]any) error {
		id := importedString(values["id"])
		name := strings.ToLower(strings.TrimSpace(firstImportedString(values, "lower_name", "name")))
		if id != "" && name != "" {
			users[id] = name
		}
		return nil
	}); err != nil {
		return nil, err
	}
	repositories := map[string]string{}
	if err := visitImportedDocuments(store, "repository", func(_ string, values map[string]any) error {
		id := importedString(values["id"])
		owner := users[importedString(values["owner_id"])]
		name := strings.ToLower(strings.TrimSpace(firstImportedString(values, "lower_name", "name")))
		if id != "" && owner != "" && name != "" {
			repositories[owner+"/"+name] = id
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return repositories, nil
}

func visitImportedDocuments(store *state.Store, table string, visit func(string, map[string]any) error) error {
	after := ""
	for {
		documents, err := store.ListPage("source-import-"+table, after, 500)
		if err != nil {
			return err
		}
		for _, document := range documents {
			decoder := json.NewDecoder(bytes.NewReader(document.Payload))
			decoder.UseNumber()
			var row importedRow
			if err := decoder.Decode(&row); err != nil || row.SourceTable != table || row.Ordinal < 1 || row.Values == nil || row.External != "" {
				return fmt.Errorf("%w: corrupt imported %s row", ErrInvalidMaterialization, table)
			}
			if err := visit(document.ID, row.Values); err != nil {
				return err
			}
		}
		if len(documents) < 500 {
			return nil
		}
		after = documents[len(documents)-1].ID
	}
}

func firstImportedString(values map[string]any, names ...string) string {
	for _, name := range names {
		if value := importedString(values[name]); value != "" {
			return value
		}
	}
	return ""
}

func importedString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	}
	return ""
}

func repositoryID(relative string, seen map[string]struct{}) string {
	base := strings.TrimSuffix(strings.ToLower(filepath.ToSlash(relative)), ".git")
	var builder strings.Builder
	lastDash := false
	for _, character := range base {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if valid {
			builder.WriteRune(character)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	id := strings.Trim(builder.String(), "-")
	digest := sha256.Sum256([]byte(relative))
	if id == "" {
		id = "repository"
	}
	if len(id) > 54 {
		id = strings.Trim(id[:54], "-")
	}
	if _, exists := seen[id]; exists {
		id += "-" + hex.EncodeToString(digest[:4])
	}
	return id
}

func repositoryDefaultBranch(repository string) string {
	payload, err := os.ReadFile(filepath.Join(repository, "HEAD"))
	if err == nil {
		const prefix = "ref: refs/heads/"
		value := strings.TrimSpace(string(payload))
		if strings.HasPrefix(value, prefix) && len(value) > len(prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return "main"
}

func copyOptionalTree(source, target string) error {
	if _, err := os.Lstat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return copyTree(source, target)
}

func copyTree(source, target string) error {
	entries := 0
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > maxMaterializedFiles {
			return fmt.Errorf("%w: asset tree exceeds file limit", ErrInvalidMaterialization)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: asset tree contains a symbolic link", ErrInvalidMaterialization)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%w: asset tree contains a special file", ErrInvalidMaterialization)
		}
		return copyFile(path, destination)
	})
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}

func writeExclusiveFile(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(payload)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(hasher, file)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func materializedTreeDigest(content string) (string, error) {
	hasher := sha256.New()
	entries := 0
	err := filepath.WalkDir(content, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == content || entry.Name() == "gitsss-materialization.json" && filepath.Dir(path) == content {
			return nil
		}
		entries++
		if entries > maxMaterializedFiles {
			return fmt.Errorf("%w: content exceeds file limit", ErrInvalidMaterialization)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: content contains a symbolic link", ErrInvalidMaterialization)
		}
		relative, err := filepath.Rel(content, path)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(hasher, filepath.ToSlash(relative)+"\x00")
		if entry.IsDir() {
			_, _ = io.WriteString(hasher, "d\x00")
			return nil
		}
		if !entry.Type().IsRegular() {
			return ErrInvalidMaterialization
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
		_, _ = io.WriteString(hasher, "\x00")
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeMaterializationManifest(content string, manifest Materialization) error {
	payload, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return writeExclusiveFile(filepath.Join(content, "gitsss-materialization.json"), payload)
}
