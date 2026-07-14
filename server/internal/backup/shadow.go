// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

const (
	maxShadowMismatchDetails = 64
	maxShadowDetailBytes     = 512
)

type ShadowFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Subject  string `json:"subject,omitempty"`
	Message  string `json:"message"`
}

type ShadowAssetComparison struct {
	Kind          string `json:"kind"`
	SourceFiles   int64  `json:"source_files"`
	SnapshotFiles int64  `json:"snapshot_files"`
	SourceBytes   int64  `json:"source_bytes"`
	SnapshotBytes int64  `json:"snapshot_bytes"`
	Matches       bool   `json:"matches"`
}

type ShadowRepositoryComparison struct {
	Path         string   `json:"path"`
	Source       GitAudit `json:"source"`
	Restored     GitAudit `json:"restored"`
	RefsMatch    bool     `json:"refs_match"`
	ObjectsMatch bool     `json:"objects_match"`
}

type ShadowRepositorySummary struct {
	Source     int `json:"source"`
	Restored   int `json:"restored"`
	Matched    int `json:"matched"`
	Mismatched int `json:"mismatched"`
}

type ShadowAudit struct {
	AssessmentID       string                       `json:"assessment_id"`
	Target             string                       `json:"target"`
	ConfigMatches      bool                         `json:"config_matches"`
	RestoreReady       bool                         `json:"restore_ready"`
	Assets             []ShadowAssetComparison      `json:"assets"`
	RepositorySummary  ShadowRepositorySummary      `json:"repository_summary"`
	Repositories       []ShadowRepositoryComparison `json:"repository_mismatches,omitempty"`
	MismatchTruncated  bool                         `json:"mismatch_truncated"`
	Findings           []ShadowFinding              `json:"findings,omitempty"`
	ReadyForShadowMode bool                         `json:"ready_for_shadow_mode"`
	AuditedAt          time.Time                    `json:"audited_at"`
}

func AuditShadowExact(inputPath, passphrase, target string, options takeover.Options, expectedAssessmentID, expectedContainerSHA256 string) (ShadowAudit, error) {
	verification, err := Verify(inputPath, passphrase)
	if err != nil {
		return ShadowAudit{}, err
	}
	if expectedContainerSHA256 == "" || verification.ContainerSHA256 != expectedContainerSHA256 {
		return ShadowAudit{}, ErrIntegrity
	}
	if expectedAssessmentID == "" || verification.Manifest.Source.AssessmentID != expectedAssessmentID {
		return ShadowAudit{}, ErrConfirmation
	}
	if options.FreezeID != "" {
		evidence, err := takeover.CheckFreeze(options)
		if err != nil {
			return ShadowAudit{}, err
		}
		if !evidence.Offline || evidence.ID != options.FreezeID || verification.Manifest.Source.FreezeID != options.FreezeID {
			return ShadowAudit{}, fmt.Errorf("%w: freeze evidence is absent, stale or no longer offline", ErrConfirmation)
		}
	}
	return auditShadowVerified(verification, target, options)
}

func auditShadowVerified(verification Verification, target string, options takeover.Options) (ShadowAudit, error) {
	if strings.TrimSpace(options.AgentID) == "" || strings.TrimSpace(options.ConfigPath) == "" {
		return ShadowAudit{}, fmt.Errorf("%w: live Agent identity and config path are required", ErrInvalidBackup)
	}
	if options.MaxEntries <= 0 || options.MaxEntries > maxManifestEntries {
		options.MaxEntries = maxManifestEntries
	}
	source, err := takeover.Assess(options)
	if err != nil {
		return ShadowAudit{}, err
	}
	restore, err := auditVerifiedRestore(verification, target)
	if err != nil {
		return ShadowAudit{}, err
	}
	audit := ShadowAudit{
		AssessmentID: verification.Manifest.Source.AssessmentID, Target: restore.Target,
		RestoreReady: restore.ReadyForShadowMode, AuditedAt: time.Now().UTC(),
	}
	audit.ConfigMatches = shadowConfigMatches(source.ConfigPath, verification.Manifest)
	if source.ID != audit.AssessmentID || !audit.ConfigMatches {
		audit.addFinding("blocker", "source.config_changed", "config", "源实例配置与已确认备份快照不一致，需要重新评估和备份")
	}
	if !restore.ReadyForShadowMode {
		audit.addFinding("blocker", "restore.not_ready", restore.Target, "隔离恢复未通过逐文件、Git 或 LFS 完整性审计")
	}
	audit.Assets = compareShadowAssets(source.Assets, verification.Manifest)
	audit.Assets = append(audit.Assets, audit.compareDatabase(source, verification.Manifest))
	sort.Slice(audit.Assets, func(left, right int) bool { return audit.Assets[left].Kind < audit.Assets[right].Kind })
	for _, asset := range audit.Assets {
		if !asset.Matches && asset.Kind != databaseSnapshotKind {
			audit.addFinding("blocker", "source.asset_changed", asset.Kind, "源资产清单相对备份快照发生变化")
		}
	}
	if err := audit.compareRepositories(source, restore); err != nil {
		return ShadowAudit{}, err
	}
	audit.ReadyForShadowMode = audit.ConfigMatches && audit.RestoreReady && audit.RepositorySummary.Mismatched == 0
	for _, finding := range audit.Findings {
		if finding.Severity == "blocker" {
			audit.ReadyForShadowMode = false
		}
	}
	return audit, nil
}

func (audit *ShadowAudit) compareRepositories(source takeover.Report, restore RestoreAudit) error {
	repositoryAsset, found := shadowAsset(source.Assets, "repositories")
	if !found || !repositoryAsset.Exists || !repositoryAsset.Readable || repositoryAsset.Truncated {
		audit.addFinding("blocker", "repositories.source_unavailable", "repositories", "源仓库目录不可完整读取")
		return nil
	}
	relativeRepositories, err := discoverShadowRepositories(repositoryAsset.Path)
	if err != nil {
		return err
	}
	gitExecutable, gitErr := exec.LookPath("git")
	sourceAudits := make(map[string]GitAudit, len(relativeRepositories))
	for _, relative := range relativeRepositories {
		if gitErr != nil {
			sourceAudits[relative] = GitAudit{Path: relative, Error: "git executable is unavailable"}
			continue
		}
		sourceAudits[relative] = auditGitRepository(gitExecutable, repositoryAsset.Path, relative)
	}
	restoredAudits := make(map[string]GitAudit, len(restore.Git))
	for _, repository := range restore.Git {
		relative, ok := shadowRestoredRepositoryPath(repository.Path)
		if !ok {
			audit.addFinding("blocker", "repositories.restore_path_invalid", repository.Path, "恢复仓库不在已验证的 repositories 命名空间")
			continue
		}
		repository.Path = relative
		if _, exists := restoredAudits[relative]; exists {
			audit.addFinding("blocker", "repositories.restore_duplicate", relative, "恢复副本包含重复仓库路径")
			continue
		}
		restoredAudits[relative] = repository
	}
	audit.RepositorySummary.Source = len(sourceAudits)
	audit.RepositorySummary.Restored = len(restoredAudits)
	paths := make(map[string]struct{}, len(sourceAudits)+len(restoredAudits))
	for repository := range sourceAudits {
		paths[repository] = struct{}{}
	}
	for repository := range restoredAudits {
		paths[repository] = struct{}{}
	}
	ordered := make([]string, 0, len(paths))
	for repository := range paths {
		ordered = append(ordered, repository)
	}
	sort.Strings(ordered)
	for _, repository := range ordered {
		sourceAudit, sourceFound := sourceAudits[repository]
		restoredAudit, restoredFound := restoredAudits[repository]
		comparison := ShadowRepositoryComparison{Path: repository, Source: sourceAudit, Restored: restoredAudit}
		comparison.RefsMatch = sourceFound && restoredFound && sourceAudit.Valid && restoredAudit.Valid && sourceAudit.Refs == restoredAudit.Refs && sourceAudit.ReferenceSHA256 == restoredAudit.ReferenceSHA256
		comparison.ObjectsMatch = sourceFound && restoredFound && sourceAudit.Valid && restoredAudit.Valid && sourceAudit.Objects == restoredAudit.Objects
		if comparison.RefsMatch {
			audit.RepositorySummary.Matched++
			if !comparison.ObjectsMatch {
				audit.addFinding("warning", "repositories.unreachable_objects_differ", repository, "Git refs 一致，但可达范围外对象数量不同")
			}
			continue
		}
		audit.RepositorySummary.Mismatched++
		if len(audit.Repositories) < maxShadowMismatchDetails {
			audit.Repositories = append(audit.Repositories, compactShadowRepositoryComparison(comparison))
		} else {
			audit.MismatchTruncated = true
		}
	}
	if audit.RepositorySummary.Mismatched > 0 {
		audit.addFinding("blocker", "repositories.refs_changed", "repositories", "源仓库集合或 refs 指纹与恢复副本不一致")
	}
	return nil
}

func compareShadowAssets(assets []takeover.Asset, manifest Manifest) []ShadowAssetComparison {
	type aggregate struct {
		files int64
		bytes int64
	}
	snapshot := make(map[string]aggregate)
	for _, entry := range manifest.Entries {
		if entry.Kind == "config" || entry.Kind == databaseSnapshotKind || entry.Kind == "database" {
			continue
		}
		value := snapshot[entry.Kind]
		value.files++
		value.bytes += entry.Size
		snapshot[entry.Kind] = value
	}
	comparisons := make([]ShadowAssetComparison, 0, len(assets)+len(snapshot))
	seen := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if asset.Kind == "database" {
			continue
		}
		value := snapshot[asset.Kind]
		comparison := ShadowAssetComparison{
			Kind: asset.Kind, SourceFiles: asset.Files, SnapshotFiles: value.files,
			SourceBytes: asset.Bytes, SnapshotBytes: value.bytes,
		}
		comparison.Matches = asset.Readable && !asset.Truncated && comparison.SourceFiles == comparison.SnapshotFiles && comparison.SourceBytes == comparison.SnapshotBytes
		if !asset.Exists && value.files == 0 {
			comparison.Matches = true
		}
		comparisons = append(comparisons, comparison)
		seen[asset.Kind] = struct{}{}
	}
	for kind, value := range snapshot {
		if _, exists := seen[kind]; exists {
			continue
		}
		comparisons = append(comparisons, ShadowAssetComparison{Kind: kind, SnapshotFiles: value.files, SnapshotBytes: value.bytes})
	}
	sort.Slice(comparisons, func(left, right int) bool { return comparisons[left].Kind < comparisons[right].Kind })
	return comparisons
}

func (audit *ShadowAudit) compareDatabase(source takeover.Report, manifest Manifest) ShadowAssetComparison {
	comparison := ShadowAssetComparison{Kind: databaseSnapshotKind}
	if source.Database.Type != "sqlite3" {
		if source.Database.Type != "postgres" && source.Database.Type != "mysql" {
			audit.addFinding("blocker", "database.snapshot_unavailable", source.Database.Type, "当前数据库类型尚无一致性 dump 适配器")
			return comparison
		}
		entry, found := manifestEntryByKind(manifest, databaseSnapshotKind)
		if !found {
			audit.addFinding("blocker", "database.snapshot_unavailable", source.Database.Type, "备份没有远程数据库一致性 dump，不能声称影子数据完整")
			return comparison
		}
		comparison.SourceFiles = 1
		comparison.SourceBytes = entry.Size
		comparison.SnapshotFiles = 1
		comparison.SnapshotBytes = entry.Size
		if source.FreezeID == "" || manifest.Source.FreezeID == "" || source.FreezeID != manifest.Source.FreezeID {
			audit.addFinding("blocker", "database.freeze_required", source.Database.Type, "远程数据库 dump 必须绑定当前停服证据")
			return comparison
		}
		connection, err := takeover.ReadDatabaseConnection(source.ConfigPath)
		if err != nil {
			audit.addFinding("blocker", "database.snapshot_failed", source.Database.Type, "无法读取源数据库连接配置")
			return comparison
		}
		temporaryDirectory, err := os.MkdirTemp("", "gitsss-shadow-database-")
		if err != nil {
			audit.addFinding("blocker", "database.snapshot_failed", source.Database.Type, "无法创建数据库影子快照")
			return comparison
		}
		defer os.RemoveAll(temporaryDirectory)
		snapshotPath := filepath.Join(temporaryDirectory, source.Database.Type+".jsonl")
		if err := snapshotEmbeddedDatabase(connection, snapshotPath); err != nil {
			audit.addFinding("blocker", "database.snapshot_failed", source.Database.Type, "无法创建数据库影子快照")
			return comparison
		}
		info, digest, err := hashFile(snapshotPath)
		if err != nil {
			audit.addFinding("blocker", "database.snapshot_failed", source.Database.Type, "无法校验数据库影子快照")
			return comparison
		}
		comparison.SourceBytes = info.Size()
		comparison.Matches = info.Size() == entry.Size && digest == entry.SHA256
		if !comparison.Matches {
			audit.addFinding("blocker", "database.source_changed", source.Database.Type, "源数据库相对冻结备份发生变化")
		}
		return comparison
	}
	entry, found := manifestEntryByKind(manifest, databaseSnapshotKind)
	if !found {
		audit.addFinding("blocker", "database.snapshot_legacy", "sqlite3", "备份未包含 SQLite 在线一致性快照，需要重新导出")
		return comparison
	}
	comparison.SnapshotFiles = 1
	comparison.SnapshotBytes = entry.Size
	database, found := shadowAsset(source.Assets, "database")
	if !found || !database.Exists || !database.Readable || database.Files != 1 {
		audit.addFinding("blocker", "database.source_unavailable", "sqlite3", "无法读取源 SQLite 数据库")
		return comparison
	}
	temporaryDirectory, err := os.MkdirTemp("", "gitsss-shadow-sqlite-")
	if err != nil {
		audit.addFinding("blocker", "database.snapshot_failed", "sqlite3", "无法创建 SQLite 影子快照")
		return comparison
	}
	defer os.RemoveAll(temporaryDirectory)
	snapshotPath := filepath.Join(temporaryDirectory, "gitea.db")
	if err := snapshotSQLite(database.Path, snapshotPath); err != nil {
		audit.addFinding("blocker", "database.snapshot_failed", "sqlite3", "无法创建 SQLite 影子快照")
		return comparison
	}
	info, digest, err := hashFile(snapshotPath)
	if err != nil {
		audit.addFinding("blocker", "database.snapshot_failed", "sqlite3", "无法校验 SQLite 影子快照")
		return comparison
	}
	comparison.SourceFiles = 1
	comparison.SourceBytes = info.Size()
	comparison.Matches = info.Size() == entry.Size && digest == entry.SHA256
	if !comparison.Matches {
		audit.addFinding("blocker", "database.source_changed", "sqlite3", "源 SQLite 数据库相对备份快照发生变化")
	}
	return comparison
}

func discoverShadowRepositories(root string) ([]string, error) {
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("%w: source repository root is unsafe", ErrInvalidBackup)
	}
	repositories := make([]string, 0)
	err = filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink found in source repositories", ErrInvalidBackup)
		}
		if !entry.IsDir() || filePath == root || !strings.HasSuffix(strings.ToLower(entry.Name()), ".git") {
			return nil
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		repositories = append(repositories, filepath.ToSlash(relative))
		if len(repositories) > maxAuditRepositories {
			return fmt.Errorf("%w: source contains more than %d repositories", ErrInvalidBackup, maxAuditRepositories)
		}
		return filepath.SkipDir
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(repositories)
	return repositories, nil
}

func shadowRestoredRepositoryPath(value string) (string, bool) {
	const prefix = "data/repositories/"
	value = path.Clean(value)
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	relative := strings.TrimPrefix(value, prefix)
	return relative, relative != "" && relative != "." && !strings.HasPrefix(relative, "../")
}

func shadowConfigMatches(configPath string, manifest Manifest) bool {
	for _, entry := range manifest.Entries {
		if entry.Kind != "config" || entry.Path != "config/app.ini" {
			continue
		}
		info, digest, err := hashFile(configPath)
		return err == nil && info.Size() == entry.Size && digest == entry.SHA256
	}
	return false
}

func manifestEntryByKind(manifest Manifest, kind string) (Entry, bool) {
	for _, entry := range manifest.Entries {
		if entry.Kind == kind {
			return entry, true
		}
	}
	return Entry{}, false
}

func shadowAsset(assets []takeover.Asset, kind string) (takeover.Asset, bool) {
	for _, asset := range assets {
		if asset.Kind == kind {
			return asset, true
		}
	}
	return takeover.Asset{}, false
}

func (audit *ShadowAudit) addFinding(severity, code, subject, message string) {
	if len(audit.Findings) >= maxShadowMismatchDetails {
		return
	}
	audit.Findings = append(audit.Findings, ShadowFinding{
		Severity: compactShadowText(severity),
		Code:     compactShadowText(code),
		Subject:  compactShadowText(subject),
		Message:  compactShadowText(message),
	})
}

func compactShadowRepositoryComparison(comparison ShadowRepositoryComparison) ShadowRepositoryComparison {
	comparison.Path = compactShadowText(comparison.Path)
	comparison.Source.Path = compactShadowText(comparison.Source.Path)
	comparison.Source.Error = compactShadowText(comparison.Source.Error)
	comparison.Restored.Path = compactShadowText(comparison.Restored.Path)
	comparison.Restored.Error = compactShadowText(comparison.Restored.Error)
	return comparison
}

func compactShadowText(value string) string {
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "�")
	}
	if len(value) <= maxShadowDetailBytes {
		return value
	}
	limit := maxShadowDetailBytes
	for limit > 0 && !utf8.ValidString(value[:limit]) {
		limit--
	}
	return value[:limit]
}
