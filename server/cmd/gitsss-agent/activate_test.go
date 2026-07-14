// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestActivationHealthArgumentsUseProtectedTokenFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "agent.token")
	arguments, err := activationHealthArguments([]string{
		"work", "--server", "https://gitsss.example/internal", "--id", "agent-1", "--token-file", tokenFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"health", "--server", "https://gitsss.example/internal", "--token-file", tokenFile}
	if !reflect.DeepEqual(arguments, expected) {
		t.Fatalf("arguments = %#v, want %#v", arguments, expected)
	}
	if _, err := activationHealthArguments([]string{"work", "--server", "https://gitsss.example", "--id", "agent-1", "--token-file", "relative.token"}); err == nil {
		t.Fatal("relative token file was accepted")
	}
	if _, err := activationHealthArguments([]string{"work", "--server", "https://gitsss.example", "--server", "https://other.example", "--id", "agent-1", "--token-file", tokenFile}); err == nil {
		t.Fatal("duplicate server argument was accepted")
	}
}
