// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestPortableDatabaseValuePreservesBinaryAndTime(t *testing.T) {
	binary, err := portableDatabaseValue([]byte{0, 1, 2, 255})
	if err != nil || binary.Kind != "bytes" || binary.Data != base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 255}) {
		t.Fatalf("binary value = %#v, %v", binary, err)
	}
	timestamp := time.Date(2026, 7, 14, 12, 30, 45, 123, time.FixedZone("CST", 8*60*60))
	encoded, err := portableDatabaseValue(timestamp)
	if err != nil || encoded.Kind != "time" || encoded.Data != timestamp.Format(time.RFC3339Nano) {
		t.Fatalf("time value = %#v, %v", encoded, err)
	}
	if _, err := portableDatabaseValue(struct{}{}); err == nil {
		t.Fatal("unsupported database value was accepted")
	}
}

func TestSourceRowsQueryQuotesDiscoveredIdentifiers(t *testing.T) {
	postgres := sourceRowsQuery("postgres", sourceTable{Schema: `public`, Name: `odd"table`}, []string{"id"})
	if !strings.Contains(postgres, `"public"."odd""table"`) || !strings.Contains(postgres, "row_to_json") {
		t.Fatalf("PostgreSQL rows query = %s", postgres)
	}
	mysql := sourceRowsQuery("mysql", sourceTable{Schema: "gitea", Name: "odd`table"}, []string{"id", "binary`value"})
	if !strings.Contains(mysql, "`gitea`.`odd``table`") || !strings.Contains(mysql, "ORDER BY `id`, `binary``value`") {
		t.Fatalf("MySQL rows query = %s", mysql)
	}
}

func TestEmbeddedDatabaseConnectorsNeedNoExternalExecutable(t *testing.T) {
	connections := []takeover.DatabaseConnection{
		{Type: "postgres", Host: "/var/run/postgresql", Name: "gitea", User: "gitsss", Password: "secret", TLSMode: "disable"},
		{Type: "mysql", Host: "[fd00::1]:3307", Name: "gitea", User: "gitsss", Password: "secret", TLSMode: "skip-verify"},
	}
	for _, connection := range connections {
		database, err := openSourceDatabase(connection)
		if err != nil {
			t.Fatalf("open %s connector: %v", connection.Type, err)
		}
		if err := database.Close(); err != nil {
			t.Fatalf("close %s connector: %v", connection.Type, err)
		}
	}
}

func TestRedactDatabaseErrorRemovesPassword(t *testing.T) {
	err := redactDatabaseError(errors.New("connect failed with super-secret"), "super-secret")
	if err == nil || strings.Contains(err.Error(), "super-secret") || !strings.Contains(err.Error(), "<redacted>") {
		t.Fatalf("redacted error = %v", err)
	}
}
