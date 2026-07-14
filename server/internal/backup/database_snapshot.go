// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

const (
	databaseSnapshotTimeout = 30 * time.Minute
	maxSnapshotTables       = 10_000
	maxSnapshotColumns      = 4_096
)

type portableSnapshotHeader struct {
	Format   string `json:"format"`
	Database string `json:"database"`
}

type portableSnapshotTable struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
	Types   []string `json:"types"`
}

type portableSnapshotRow struct {
	Table  string                  `json:"table"`
	Values []portableSnapshotValue `json:"values"`
}

type portableSnapshotValue struct {
	Kind string `json:"kind"`
	Data string `json:"data,omitempty"`
}

type sourceTable struct {
	Schema string
	Name   string
}

func prepareNativeDatabasePlan(report takeover.Report, now time.Time) (*PreparedPlan, error) {
	connection, err := takeover.ReadDatabaseConnection(report.ConfigPath)
	if err != nil {
		return nil, err
	}
	if connection.Type != report.Database.Type || connection.Name == "" || connection.User == "" {
		return nil, fmt.Errorf("%w: database type, name and user are required", ErrInvalidBackup)
	}
	temporaryDirectory, err := os.MkdirTemp("", "gitsss-database-snapshot-")
	if err != nil {
		return nil, err
	}
	prepared := &PreparedPlan{temporaryDirectory: temporaryDirectory}
	cleanup := func() {
		_ = prepared.Close()
	}
	snapshotPath := filepath.Join(temporaryDirectory, connection.Type+".jsonl")
	if err := snapshotEmbeddedDatabase(connection, snapshotPath); err != nil {
		cleanup()
		return nil, err
	}
	info, err := os.Stat(snapshotPath)
	if err != nil {
		cleanup()
		return nil, err
	}
	snapshotReport := report
	snapshotReport.Assets = append(append([]takeover.Asset(nil), report.Assets...), takeover.Asset{
		Kind: databaseSnapshotKind, Path: snapshotPath, Exists: true, Readable: true,
		Files: 1, Bytes: info.Size(),
	})
	prepared.Plan, err = BuildPlan(snapshotReport, now)
	if err != nil {
		cleanup()
		return nil, err
	}
	return prepared, nil
}

func snapshotEmbeddedDatabase(connection takeover.DatabaseConnection, destination string) error {
	if connection.Type != "postgres" && connection.Type != "mysql" {
		return fmt.Errorf("%w: unsupported embedded source driver %q", ErrInvalidBackup, connection.Type)
	}
	if connection.Name == "" || connection.User == "" {
		return fmt.Errorf("%w: database name and user are required", ErrInvalidBackup)
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("%w: database snapshot already exists", ErrInvalidBackup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), databaseSnapshotTimeout)
	defer cancel()
	database, err := openSourceDatabase(connection)
	if err != nil {
		return redactDatabaseError(err, connection.Password)
	}
	defer database.Close()
	transaction, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return redactDatabaseError(err, connection.Password)
	}
	defer transaction.Rollback()
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	completed := false
	defer func() {
		if !completed {
			file.Close()
			_ = os.Remove(destination)
		}
	}()
	writer := bufio.NewWriterSize(file, 256<<10)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(portableSnapshotHeader{Format: "gitsss-database-v1", Database: connection.Type}); err != nil {
		return err
	}
	tables, err := listSourceTables(ctx, transaction, connection.Type)
	if err != nil {
		return redactDatabaseError(err, connection.Password)
	}
	for _, table := range tables {
		if err := snapshotSourceTable(ctx, transaction, connection.Type, table, encoder); err != nil {
			return redactDatabaseError(err, connection.Password)
		}
	}
	if err := transaction.Commit(); err != nil {
		return redactDatabaseError(err, connection.Password)
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	completed = true
	return nil
}

func openSourceDatabase(connection takeover.DatabaseConnection) (*sql.DB, error) {
	host, port, socket := splitSourceDatabaseHost(connection.Host, defaultDatabasePort(connection.Type))
	switch connection.Type {
	case "postgres":
		uri := &url.URL{
			Scheme: "postgres", User: url.UserPassword(connection.User, connection.Password),
			Path: "/" + connection.Name,
		}
		query := uri.Query()
		if socket != "" {
			query.Set("host", socket)
			query.Set("port", port)
		} else {
			uri.Host = net.JoinHostPort(host, port)
		}
		if connection.TLSMode != "" {
			query.Set("sslmode", connection.TLSMode)
		}
		query.Set("connect_timeout", "15")
		uri.RawQuery = query.Encode()
		configuration, err := pgx.ParseConfig(uri.String())
		if err != nil {
			return nil, err
		}
		database := stdlib.OpenDB(*configuration)
		database.SetMaxOpenConns(1)
		return database, nil
	case "mysql":
		network := "tcp"
		address := net.JoinHostPort(host, port)
		if socket != "" {
			network = "unix"
			address = socket
		}
		configuration := mysqldriver.NewConfig()
		configuration.User = connection.User
		configuration.Passwd = connection.Password
		configuration.Net = network
		configuration.Addr = address
		configuration.DBName = connection.Name
		configuration.Timeout = 15 * time.Second
		configuration.ReadTimeout = databaseSnapshotTimeout
		configuration.WriteTimeout = 30 * time.Second
		configuration.TLSConfig = mysqlDriverTLSMode(connection.TLSMode)
		connector, err := mysqldriver.NewConnector(configuration)
		if err != nil {
			return nil, err
		}
		database := sql.OpenDB(connector)
		database.SetMaxOpenConns(1)
		return database, nil
	default:
		return nil, fmt.Errorf("%w: unsupported embedded source driver %q", ErrInvalidBackup, connection.Type)
	}
}

func listSourceTables(ctx context.Context, transaction *sql.Tx, databaseType string) ([]sourceTable, error) {
	query := `SELECT table_schema, table_name FROM information_schema.tables WHERE table_type = 'BASE TABLE'`
	if databaseType == "postgres" {
		query += ` AND table_schema <> 'information_schema' AND table_schema NOT LIKE 'pg_%' ORDER BY table_schema, table_name`
	} else {
		query += ` AND table_schema = DATABASE() ORDER BY table_schema, table_name`
	}
	rows, err := transaction.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := make([]sourceTable, 0)
	for rows.Next() {
		var table sourceTable
		if err := rows.Scan(&table.Schema, &table.Name); err != nil {
			return nil, err
		}
		tables = append(tables, table)
		if len(tables) > maxSnapshotTables {
			return nil, fmt.Errorf("%w: database contains more than %d tables", ErrInvalidBackup, maxSnapshotTables)
		}
	}
	return tables, rows.Err()
}

func snapshotSourceTable(ctx context.Context, transaction *sql.Tx, databaseType string, table sourceTable, encoder *json.Encoder) error {
	columns, types, err := sourceTableColumns(ctx, transaction, databaseType, table)
	if err != nil {
		return err
	}
	if len(columns) == 0 || len(columns) > maxSnapshotColumns {
		return fmt.Errorf("%w: table %s.%s has invalid column count", ErrInvalidBackup, table.Schema, table.Name)
	}
	tableName := table.Schema + "." + table.Name
	if err := encoder.Encode(portableSnapshotTable{Table: tableName, Columns: columns, Types: types}); err != nil {
		return err
	}
	query := sourceRowsQuery(databaseType, table, columns)
	rows, err := transaction.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for index := range values {
		pointers[index] = &values[index]
	}
	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		encoded := make([]portableSnapshotValue, len(values))
		for index, value := range values {
			encoded[index], err = portableDatabaseValue(value)
			if err != nil {
				return fmt.Errorf("encode %s column %s: %w", tableName, columns[index], err)
			}
		}
		if err := encoder.Encode(portableSnapshotRow{Table: tableName, Values: encoded}); err != nil {
			return err
		}
	}
	return rows.Err()
}

func sourceTableColumns(ctx context.Context, transaction *sql.Tx, databaseType string, table sourceTable) ([]string, []string, error) {
	query := `SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`
	arguments := []any{table.Schema, table.Name}
	if databaseType == "postgres" {
		query = `SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`
	}
	rows, err := transaction.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	columns := make([]string, 0)
	types := make([]string, 0)
	for rows.Next() {
		var column, columnType string
		if err := rows.Scan(&column, &columnType); err != nil {
			return nil, nil, err
		}
		columns = append(columns, column)
		types = append(types, columnType)
	}
	return columns, types, rows.Err()
}

func sourceRowsQuery(databaseType string, table sourceTable, columns []string) string {
	qualified := quoteDatabaseIdentifier(databaseType, table.Schema) + "." + quoteDatabaseIdentifier(databaseType, table.Name)
	if databaseType == "postgres" {
		return "SELECT * FROM " + qualified + " AS snapshot_row ORDER BY md5(row_to_json(snapshot_row)::text), row_to_json(snapshot_row)::text"
	}
	ordered := make([]string, len(columns))
	for index, column := range columns {
		ordered[index] = quoteDatabaseIdentifier(databaseType, column)
	}
	return "SELECT * FROM " + qualified + " ORDER BY " + strings.Join(ordered, ", ")
}

func quoteDatabaseIdentifier(databaseType, value string) string {
	if databaseType == "mysql" {
		return "`" + strings.ReplaceAll(value, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func portableDatabaseValue(value any) (portableSnapshotValue, error) {
	switch typed := value.(type) {
	case nil:
		return portableSnapshotValue{Kind: "null"}, nil
	case []byte:
		return portableSnapshotValue{Kind: "bytes", Data: base64.StdEncoding.EncodeToString(typed)}, nil
	case string:
		return portableSnapshotValue{Kind: "string", Data: base64.StdEncoding.EncodeToString([]byte(typed))}, nil
	case bool:
		return portableSnapshotValue{Kind: "bool", Data: strconv.FormatBool(typed)}, nil
	case int64:
		return portableSnapshotValue{Kind: "int", Data: strconv.FormatInt(typed, 10)}, nil
	case int8:
		return portableSnapshotValue{Kind: "int", Data: strconv.FormatInt(int64(typed), 10)}, nil
	case int16:
		return portableSnapshotValue{Kind: "int", Data: strconv.FormatInt(int64(typed), 10)}, nil
	case int32:
		return portableSnapshotValue{Kind: "int", Data: strconv.FormatInt(int64(typed), 10)}, nil
	case int:
		return portableSnapshotValue{Kind: "int", Data: strconv.Itoa(typed)}, nil
	case uint64:
		return portableSnapshotValue{Kind: "uint", Data: strconv.FormatUint(typed, 10)}, nil
	case uint32:
		return portableSnapshotValue{Kind: "uint", Data: strconv.FormatUint(uint64(typed), 10)}, nil
	case uint16:
		return portableSnapshotValue{Kind: "uint", Data: strconv.FormatUint(uint64(typed), 10)}, nil
	case uint8:
		return portableSnapshotValue{Kind: "uint", Data: strconv.FormatUint(uint64(typed), 10)}, nil
	case uint:
		return portableSnapshotValue{Kind: "uint", Data: strconv.FormatUint(uint64(typed), 10)}, nil
	case float64:
		return portableSnapshotValue{Kind: "float", Data: strconv.FormatFloat(typed, 'g', -1, 64)}, nil
	case float32:
		return portableSnapshotValue{Kind: "float", Data: strconv.FormatFloat(float64(typed), 'g', -1, 32)}, nil
	case time.Time:
		return portableSnapshotValue{Kind: "time", Data: typed.Format(time.RFC3339Nano)}, nil
	default:
		return portableSnapshotValue{}, fmt.Errorf("unsupported database value type %T", value)
	}
}

func splitSourceDatabaseHost(value, defaultPort string) (host, port, socket string) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "/") {
		return "localhost", defaultPort, value
	}
	if value == "" {
		return "127.0.0.1", defaultPort, ""
	}
	if parsedHost, parsedPort, err := net.SplitHostPort(value); err == nil {
		return parsedHost, normalizedSourcePort(parsedPort, defaultPort), ""
	}
	if strings.Count(value, ":") == 1 {
		parsedHost, parsedPort, found := strings.Cut(value, ":")
		if found {
			return parsedHost, normalizedSourcePort(parsedPort, defaultPort), ""
		}
	}
	return strings.Trim(value, "[]"), defaultPort, ""
}

func normalizedSourcePort(value, fallback string) string {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fallback
	}
	return strconv.Itoa(port)
}

func defaultDatabasePort(databaseType string) string {
	if databaseType == "postgres" {
		return "5432"
	}
	return "3306"
}

func mysqlDriverTLSMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "required", "require", "verify-ca", "verify-full", "verify-identity":
		return "true"
	case "skip-verify":
		return "skip-verify"
	case "preferred":
		return "preferred"
	default:
		return ""
	}
}

func redactDatabaseError(err error, password string) error {
	if err == nil || password == "" {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), password, "<redacted>"))
}
