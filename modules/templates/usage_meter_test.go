// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"
)

type meterLocale struct{}

func (meterLocale) Tr(string) string { return "Unavailable" }

func TestUsageMeterKnownAndUnknownCapacity(t *testing.T) {
	source, err := os.ReadFile("../../templates/admin/usage_meter.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	funcs := newFuncMapWebPage()
	funcs["ctx"] = func() any { return struct{ Locale meterLocale }{} }
	tmpl, err := template.New("meter").Funcs(funcs).Parse(string(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		used, capacity int64
		expected       string
	}{{0, 0, "Unavailable"}, {0, 100, "0.0%"}, {50, 100, "50.0%"}, {100, 100, "100.0%"}} {
		var output bytes.Buffer
		if err := tmpl.Execute(&output, map[string]any{"Label": "Disk", "Used": test.used, "Capacity": test.capacity}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), test.expected) || (test.capacity == 0 && strings.Contains(output.String(), "<progress")) {
			t.Fatalf("invalid meter: %s", output.String())
		}
	}
}
