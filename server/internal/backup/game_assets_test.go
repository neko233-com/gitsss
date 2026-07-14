// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestGameAssetsSurviveEncryptedBackupAndRestore(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	assetsPath := filepath.Join(directory, "game.git")
	if err := os.Mkdir(assetsPath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, configPath, "[database]\nDB_TYPE = sqlite3\n")
	assets := map[string][]byte{
		"concept.psd": append([]byte("8BPS\x00\x01"), bytes.Repeat([]byte{0x19, 0x96, 0x02, 0x29}, 4096)...),
		"level.fbx":   append([]byte("Kaydara FBX Binary  \x00\x1a\x00"), bytes.Repeat([]byte{0xfb, 0x78, 0x00, 0x01}, 4096)...),
		"music.mp3":   append([]byte("ID3\x04\x00\x00"), bytes.Repeat([]byte{0xff, 0xfb, 0x90, 0x64}, 4096)...),
		"trailer.mp4": append([]byte("\x00\x00\x00\x18ftypisom"), bytes.Repeat([]byte{0x6d, 0x64, 0x61, 0x74}, 4096)...),
	}
	for name, content := range assets {
		writeTestBytes(t, filepath.Join(assetsPath, name), content)
	}
	report := takeover.Report{
		ID: "assessment-game-assets", ConfigPath: configPath,
		ConfigDigest: "sha256:" + strings.Repeat("a", 64),
		Assets:       []takeover.Asset{{Kind: "repositories", Path: assetsPath, Exists: true, Readable: true}},
	}
	started := time.Now()
	plan, err := BuildPlan(report, started)
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(directory, "game-assets.gitsss-backup")
	const passphrase = "game asset test passphrase"
	if err := Export(backupPath, passphrase, plan); err != nil {
		t.Fatal(err)
	}
	verification, err := Verify(backupPath, passphrase)
	if err != nil || verification.Files != len(assets)+1 {
		t.Fatalf("game asset verification = %#v, %v", verification, err)
	}
	targetPath := filepath.Join(directory, "isolated")
	if _, err := RestoreIsolatedExact(backupPath, passphrase, targetPath, report.ID, verification.ContainerSHA256); err != nil {
		t.Fatal(err)
	}
	for name, expected := range assets {
		actual, err := os.ReadFile(filepath.Join(targetPath, "content", "data", "repositories", name))
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("restored %s differs: %d bytes, %v", name, len(actual), err)
		}
	}
	t.Logf("verified PSD, FBX, MP3 and MP4 in %s", time.Since(started))
}
