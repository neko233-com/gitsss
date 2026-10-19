// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package i18n

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocaleMissingEmptyAndLateKeys(t *testing.T) {
	store := NewLocaleStore()
	if err := store.AddLocaleByJSON("en", "English", []byte(`{"a":"English","empty":"fallback"}`), nil); err != nil {
		t.Fatal(err)
	}
	store.SetDefaultLang("en")
	if err := store.AddLocaleByJSON("zh", "Chinese", []byte(`{"a":"中文","empty":"","section":{"nested":"嵌套"},"late":"later"}`), []byte(`{"a":"override"}`)); err != nil {
		t.Fatal(err)
	}
	en, _ := store.Locale("en")
	zh, _ := store.Locale("zh")
	if !zh.HasKey("empty") || en.HasKey("late") || zh.HasKey("missing") || en.TrString("late") != "late" || zh.TrString("empty") != "fallback" || zh.TrString("a") != "override" || zh.TrString("section.nested") != "嵌套" {
		t.Fatal("locale presence, overrides or fallback changed")
	}
}

func BenchmarkAllLocales(b *testing.B) {
	names, err := filepath.Glob("../../../options/locale/locale_*.json")
	if err != nil || len(names) < 20 {
		b.Fatalf("locale fixture: %v, %d", err, len(names))
	}
	sources := make(map[string][]byte, len(names))
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			b.Fatal(err)
		}
		sources[name] = data
	}
	load := func() LocaleStore {
		store := NewLocaleStore()
		for _, name := range names {
			lang := strings.TrimSuffix(filepath.Base(name), ".json")
			if err := store.AddLocaleByJSON(lang, lang, sources[name], nil); err != nil {
				b.Fatal(err)
			}
		}
		return store
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	store := load()
	runtime.GC()
	runtime.ReadMemStats(&after)
	retained := after.HeapAlloc - before.HeapAlloc
	runtime.KeepAlive(store)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		runtime.KeepAlive(load())
	}
	b.ReportMetric(float64(retained), "retained-B")
}
