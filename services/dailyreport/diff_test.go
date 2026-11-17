// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"

	"github.com/stretchr/testify/require"
)

type diffTestRepo string

func (r diffTestRepo) RelativePath() string { return string(r) }

func TestReadCommitDiff(t *testing.T) {
	root := t.TempDir()
	oldRoot := setting.RepoRootPath
	oldHome := setting.Git.HomePath
	setting.RepoRootPath = root
	setting.Git.HomePath = t.TempDir()
	t.Cleanup(func() { setting.RepoRootPath, setting.Git.HomePath = oldRoot, oldHome })
	dir := filepath.Join(root, "repo")
	require.NoError(t, os.Mkdir(dir, 0o755))
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return strings.TrimSpace(string(output))
	}
	git("init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "combat.txt"), []byte("old combat\n"), 0o644))
	git("add", ".")
	git("commit", "-m", "unrelated message")
	rootID := git("rev-parse", "HEAD")
	rootDiff, truncated, err := readCommitDiff(t.Context(), diffTestRepo("repo"), rootID, maxCommitDiffBytes)
	require.NoError(t, err)
	require.False(t, truncated)
	require.Contains(t, rootDiff, "+old combat")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "combat.txt"), []byte("new combat behaviour\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "asset.bin"), []byte{0, 1, 2, 3}, 0o644))
	git("add", ".")
	git("commit", "-m", "documentation only")
	id := git("rev-parse", "HEAD")
	diff, truncated, err := readCommitDiff(t.Context(), diffTestRepo("repo"), id, maxCommitDiffBytes)
	require.NoError(t, err)
	require.False(t, truncated)
	require.Contains(t, diff, "-old combat")
	require.Contains(t, diff, "+new combat behaviour")
	require.Contains(t, diff, "Binary files")
	require.NotContains(t, diff, "documentation only")

	diff, truncated, err = readCommitDiff(t.Context(), diffTestRepo("repo"), id, 64)
	require.NoError(t, err)
	require.True(t, truncated)
	require.LessOrEqual(t, len(diff), 66)
	branch := git("branch", "--show-current")
	git("checkout", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature change\n"), 0o644))
	git("add", ".")
	git("commit", "-m", "feature")
	git("checkout", branch)
	git("merge", "--no-ff", "feature", "-m", "merge feature")
	mergeID := git("rev-parse", "HEAD")
	mergeDiff, truncated, err := readCommitDiff(t.Context(), diffTestRepo("repo"), mergeID, maxCommitDiffBytes)
	require.NoError(t, err)
	require.False(t, truncated)
	require.Contains(t, mergeDiff, "+feature change")
	git("commit", "--allow-empty", "-m", "empty")
	emptyID := git("rev-parse", "HEAD")
	empty, truncated, err := readCommitDiff(t.Context(), diffTestRepo("repo"), emptyID, maxCommitDiffBytes)
	require.NoError(t, err)
	require.Empty(t, empty)
	require.False(t, truncated)
	_, _, err = readCommitDiff(t.Context(), diffTestRepo("repo"), "--help", 64)
	require.Error(t, err)
	_, _, err = readCommitDiff(t.Context(), diffTestRepo("repo"), strings.Repeat("a", 40), 64)
	require.Error(t, err)
	blobID := git("rev-parse", "HEAD:asset.bin")
	_, _, err = readCommitDiff(t.Context(), diffTestRepo("repo"), blobID, 64)
	require.Error(t, err)
}
