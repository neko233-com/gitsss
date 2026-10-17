// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/neko233-com/gitsss/modules/json"
)

//go:embed builtin/*.png builtin/manifest.json
var builtinAssets embed.FS

type BuiltinStyle struct {
	ID      string
	Label   string
	Indexes []int
}

var builtinCatalog = readBuiltinCatalog()

var BuiltinCount = func() int {
	count := 0
	for _, style := range builtinCatalog {
		count += len(style.Indexes)
	}
	return count
}()

func readBuiltinCatalog() []BuiltinStyle {
	data, err := builtinAssets.ReadFile("builtin/manifest.json")
	if err != nil {
		panic(err)
	}
	manifest := struct {
		Styles []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"styles"`
		Assets []struct {
			File  string `json:"file"`
			Style string `json:"style"`
		} `json:"assets"`
	}{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		panic(err)
	}
	styles := []BuiltinStyle{}
	for _, style := range manifest.Styles {
		styles = append(styles, BuiltinStyle{ID: style.ID, Label: style.Label})
	}
	for index, asset := range manifest.Assets {
		if asset.File != fmt.Sprintf("%02d.png", index) {
			panic("built-in avatar indexes must remain contiguous and stable")
		}
		found := false
		for i := range styles {
			if styles[i].ID == asset.Style {
				styles[i].Indexes = append(styles[i].Indexes, index)
				found = true
				break
			}
		}
		if !found {
			styles = append(styles, BuiltinStyle{ID: asset.Style, Label: strings.ReplaceAll(asset.Style, "-", " "), Indexes: []int{index}})
		}
	}
	return styles
}

func BuiltinStyles() []BuiltinStyle {
	styles := make([]BuiltinStyle, len(builtinCatalog))
	for i, style := range builtinCatalog {
		styles[i] = style
		styles[i].Indexes = append([]int(nil), style.Indexes...)
	}
	return styles
}

// BuiltinPNG returns a locally embedded, immutable avatar; no external service is needed.
func BuiltinPNG(index int) ([]byte, error) {
	if index < 0 || index >= BuiltinCount {
		return nil, errors.New("invalid built-in avatar")
	}
	return builtinAssets.ReadFile(fmt.Sprintf("builtin/%02d.png", index))
}
