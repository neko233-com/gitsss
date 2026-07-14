// Copyright 2026 gitsss Authors. All rights reserved.

package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestServiceReadsTreeFileAndSearchesText(t *testing.T) {
	repositoryPath := createTestRepository(t)
	service, err := NewService([]Repository{{ID: "game", Name: "game", Path: filepath.Join(repositoryPath, "assets"), DefaultBranch: "main"}})
	if err != nil {
		t.Fatal(err)
	}

	tree, err := service.Tree(context.Background(), "game", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Entries) != 2 || tree.Entries[0].Path != "README.md" || !tree.Entries[1].BinaryAsset {
		t.Fatalf("tree entries = %#v, want README and binary asset", tree.Entries)
	}

	file, err := service.File(context.Background(), "game", "HEAD", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !file.PreviewAvailable || file.Content != "# Game\nship fast\n" {
		t.Fatalf("file = %#v, want text preview", file)
	}

	binary, err := service.File(context.Background(), "game", "HEAD", "assets/scene.fbx")
	if err != nil {
		t.Fatal(err)
	}
	if !binary.Binary || !binary.BinaryAsset || binary.PreviewAvailable {
		t.Fatalf("binary = %#v, want binary metadata only", binary)
	}

	matches, err := service.Search(context.Background(), "game", "HEAD", "ship", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Path != "README.md" || matches[0].Line != 2 {
		t.Fatalf("matches = %#v, want README line 2", matches)
	}
}

func TestServiceRejectsUnknownRepositoryAndUnsafePath(t *testing.T) {
	service, err := NewService([]Repository{{ID: "game", Name: "game", Path: createTestRepository(t)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Tree(context.Background(), "missing", "HEAD"); err != ErrNotFound {
		t.Fatalf("Tree error = %v, want ErrNotFound", err)
	}
	if _, err := service.File(context.Background(), "game", "HEAD", "../secret"); err != ErrInvalidPath {
		t.Fatalf("File error = %v, want ErrInvalidPath", err)
	}
}

func createTestRepository(t *testing.T) string {
	t.Helper()
	repositoryPath := t.TempDir()
	runGitTest(t, repositoryPath, "init", "-b", "main")
	runGitTest(t, repositoryPath, "config", "user.name", "gitsss Test")
	runGitTest(t, repositoryPath, "config", "user.email", "test@gitsss.invalid")
	if err := os.WriteFile(filepath.Join(repositoryPath, "README.md"), []byte("# Game\nship fast\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repositoryPath, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryPath, "assets", "scene.fbx"), []byte{'F', 'B', 'X', 0, 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, repositoryPath, "add", ".")
	runGitTest(t, repositoryPath, "commit", "-m", "test fixture")
	return repositoryPath
}

func runGitTest(t *testing.T, repositoryPath string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repositoryPath}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
