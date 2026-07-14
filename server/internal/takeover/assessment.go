// Copyright 2026 gitsss Authors. All rights reserved.

package takeover

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxConfigBytes = 2 << 20

var (
	ErrConfigTooLarge = errors.New("Gitea configuration exceeds 2 MiB")
	ErrInvalidConfig  = errors.New("invalid Gitea configuration")
)

type Options struct {
	AgentID      string           `json:"agent_id"`
	ConfigPath   string           `json:"config_path"`
	WorkPath     string           `json:"work_path,omitempty"`
	GiteaVersion string           `json:"gitea_version,omitempty"`
	MaxEntries   int              `json:"max_entries,omitempty"`
	Now          func() time.Time `json:"-"`
}

type Database struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Location string `json:"location"`
	TLSMode  string `json:"tls_mode,omitempty"`
}

type Asset struct {
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Readable  bool   `json:"readable"`
	Files     int64  `json:"files"`
	Bytes     int64  `json:"bytes"`
	Truncated bool   `json:"truncated"`
}

type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type Report struct {
	ID               string    `json:"id"`
	AgentID          string    `json:"agent_id"`
	ConfigPath       string    `json:"config_path"`
	ConfigDigest     string    `json:"config_digest"`
	GiteaVersion     string    `json:"gitea_version,omitempty"`
	PublicOrigin     string    `json:"public_origin,omitempty"`
	Database         Database  `json:"database"`
	Assets           []Asset   `json:"assets"`
	ActionsEnabled   bool      `json:"actions_enabled"`
	SecretFieldCount int       `json:"secret_field_count"`
	Findings         []Finding `json:"findings"`
	AssessedAt       time.Time `json:"assessed_at"`
}

func Assess(options Options) (Report, error) {
	if strings.TrimSpace(options.AgentID) == "" || strings.TrimSpace(options.ConfigPath) == "" {
		return Report{}, fmt.Errorf("%w: agent ID and config path are required", ErrInvalidConfig)
	}
	path, err := filepath.Abs(options.ConfigPath)
	if err != nil {
		return Report{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Report{}, err
	}
	if !info.Mode().IsRegular() {
		return Report{}, fmt.Errorf("%w: config must be a regular file", ErrInvalidConfig)
	}
	if info.Size() > maxConfigBytes {
		return Report{}, ErrConfigTooLarge
	}
	configuration, err := parseINI(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return Report{}, err
	}

	workPath := resolveWorkPath(path, options.WorkPath)
	appData := resolvePath(workPath, value(configuration, "server", "APP_DATA_PATH"), "data")
	assets := []Asset{
		inspect("repositories", resolvePath(workPath, value(configuration, "repository", "ROOT"), filepath.Join(appData, "gitea-repositories")), options.MaxEntries),
		inspect("lfs", resolvePath(appData, value(configuration, "lfs", "PATH"), "lfs"), options.MaxEntries),
		inspect("attachments", resolvePath(appData, value(configuration, "attachment", "PATH"), "attachments"), options.MaxEntries),
		inspect("packages", resolvePath(appData, value(configuration, "packages", "PATH"), "packages"), options.MaxEntries),
		inspect("actions-log", resolvePath(appData, value(configuration, "actions_log", "PATH"), "actions_log"), options.MaxEntries),
		inspect("actions-artifacts", resolvePath(appData, value(configuration, "actions_artifacts", "PATH"), "actions_artifacts"), options.MaxEntries),
	}
	database := databaseSummary(configuration, appData, options.MaxEntries, &assets)
	actionsEnabled := boolValue(value(configuration, "actions", "ENABLED"), true)
	digest, secretCount := configurationDigest(configuration)
	assessedAt := time.Now().UTC()
	if options.Now != nil {
		assessedAt = options.Now().UTC()
	}
	idHash := sha256.Sum256([]byte(options.AgentID + "\x00" + path + "\x00" + digest))
	report := Report{
		ID:               "assessment-" + hex.EncodeToString(idHash[:8]),
		AgentID:          strings.TrimSpace(options.AgentID),
		ConfigPath:       path,
		ConfigDigest:     digest,
		GiteaVersion:     strings.TrimSpace(options.GiteaVersion),
		PublicOrigin:     safeOrigin(value(configuration, "server", "ROOT_URL")),
		Database:         database,
		Assets:           assets,
		ActionsEnabled:   actionsEnabled,
		SecretFieldCount: secretCount,
		AssessedAt:       assessedAt,
	}
	report.Findings = findings(report)
	return report, nil
}

func DiscoverConfigPaths() []string {
	candidates := []string{
		filepath.Join("custom", "conf", "app.ini"),
		filepath.Join(string(filepath.Separator), "etc", "gitea", "app.ini"),
		filepath.Join(string(filepath.Separator), "etc", "gitea", "conf", "app.ini"),
		filepath.Join(string(filepath.Separator), "data", "gitea", "conf", "app.ini"),
	}
	if runtime.GOOS == "windows" {
		if programData := os.Getenv("ProgramData"); programData != "" {
			candidates = append(candidates, filepath.Join(programData, "gitea", "conf", "app.ini"))
		}
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "custom", "conf", "app.ini"))
	}
	seen := map[string]struct{}{}
	found := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, ok := seen[absolute]; ok {
			continue
		}
		if info, err := os.Stat(absolute); err == nil && info.Mode().IsRegular() {
			seen[absolute] = struct{}{}
			found = append(found, absolute)
		}
	}
	sort.Strings(found)
	return found
}

type ini map[string]map[string]string

func parseINI(reader io.Reader) (ini, error) {
	result := ini{"": {}}
	section := ""
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), maxConfigBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if section == "" {
				return nil, fmt.Errorf("%w: empty section", ErrInvalidConfig)
			}
			if result[section] == nil {
				result[section] = map[string]string{}
			}
			continue
		}
		key, rawValue, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%w: malformed setting", ErrInvalidConfig)
		}
		result[section][strings.ToUpper(strings.TrimSpace(key))] = cleanValue(rawValue)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	quote := rune(0)
	for index, character := range value {
		switch character {
		case '\'', '"', '`':
			if quote == 0 {
				quote = character
			} else if quote == character {
				quote = 0
			}
		case ';', '#':
			if quote == 0 && (index == 0 || value[index-1] == ' ' || value[index-1] == '\t') {
				value = strings.TrimSpace(value[:index])
				return strings.Trim(value, "'\"`")
			}
		}
	}
	return strings.Trim(value, "'\"`")
}

func value(configuration ini, section, key string) string {
	return configuration[strings.ToLower(section)][strings.ToUpper(key)]
}

func resolveWorkPath(configPath, configured string) string {
	if strings.TrimSpace(configured) != "" {
		if absolute, err := filepath.Abs(configured); err == nil {
			return absolute
		}
	}
	directory := filepath.Dir(configPath)
	if strings.EqualFold(filepath.Base(directory), "conf") && strings.EqualFold(filepath.Base(filepath.Dir(directory)), "custom") {
		return filepath.Dir(filepath.Dir(directory))
	}
	return directory
}

func resolvePath(base, configured, fallback string) string {
	path := strings.TrimSpace(configured)
	if path == "" {
		path = fallback
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

func inspect(kind, path string, maxEntries int) Asset {
	if maxEntries <= 0 || maxEntries > 1_000_000 {
		maxEntries = 10_000
	}
	asset := Asset{Kind: kind, Path: filepath.Clean(path)}
	info, err := os.Stat(path)
	if err != nil {
		return asset
	}
	asset.Exists = true
	if !info.IsDir() {
		asset.Readable = true
		asset.Files = 1
		asset.Bytes = info.Size()
		return asset
	}
	err = filepath.WalkDir(path, func(_ string, entry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		asset.Files++
		if info, infoError := entry.Info(); infoError == nil {
			asset.Bytes += info.Size()
		}
		if asset.Files >= int64(maxEntries) {
			asset.Truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	asset.Readable = err == nil
	return asset
}

func databaseSummary(configuration ini, appData string, maxEntries int, assets *[]Asset) Database {
	databaseType := strings.ToLower(strings.TrimSpace(value(configuration, "database", "DB_TYPE")))
	if databaseType == "" {
		databaseType = "sqlite3"
	}
	database := Database{Type: databaseType, Name: value(configuration, "database", "NAME"), TLSMode: value(configuration, "database", "SSL_MODE")}
	if databaseType == "sqlite3" {
		path := resolvePath(appData, value(configuration, "database", "PATH"), "gitea.db")
		database.Location = "local-file"
		*assets = append(*assets, inspect("database", path, maxEntries))
		return database
	}
	host := strings.ToLower(value(configuration, "database", "HOST"))
	if strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "/") {
		database.Location = "local-service"
	} else {
		database.Location = "remote-service"
	}
	return database
}

func boolValue(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func configurationDigest(configuration ini) (string, int) {
	lines := make([]string, 0)
	secretCount := 0
	for section, settings := range configuration {
		for key, setting := range settings {
			if secretKey(key) {
				setting = "<redacted>"
				secretCount++
			}
			lines = append(lines, section+"."+key+"="+setting)
		}
	}
	sort.Strings(lines)
	digest := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "sha256:" + hex.EncodeToString(digest[:]), secretCount
}

func secretKey(key string) bool {
	upper := strings.ToUpper(key)
	return upper == "PASSWD" || strings.Contains(upper, "PASSWORD") || strings.HasSuffix(upper, "SECRET") || strings.HasSuffix(upper, "TOKEN") || strings.Contains(upper, "SECRET_KEY")
}

func safeOrigin(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func findings(report Report) []Finding {
	items := make([]Finding, 0)
	if report.Database.Type != "sqlite3" && report.Database.Type != "mysql" && report.Database.Type != "postgres" && report.Database.Type != "mssql" {
		items = append(items, Finding{Severity: "blocker", Code: "database.unsupported", Message: "数据库类型尚未进入接管兼容矩阵"})
	}
	if strings.HasPrefix(report.PublicOrigin, "http://") {
		items = append(items, Finding{Severity: "warning", Code: "server.insecure_origin", Message: "对外地址未使用 HTTPS，切换前需要确认反向代理终止 TLS"})
	}
	if !report.ActionsEnabled {
		items = append(items, Finding{Severity: "info", Code: "actions.disabled", Message: "原实例未启用 Actions，内置 Runner 将保持未注册"})
	}
	for _, asset := range report.Assets {
		if !asset.Exists {
			items = append(items, Finding{Severity: "warning", Code: "asset.missing." + asset.Kind, Message: asset.Kind + " 路径不存在，需要确认配置覆盖或挂载"})
		} else if !asset.Readable {
			items = append(items, Finding{Severity: "blocker", Code: "asset.unreadable." + asset.Kind, Message: asset.Kind + " 路径不可完整读取"})
		} else if asset.Truncated {
			items = append(items, Finding{Severity: "info", Code: "asset.scan_truncated." + asset.Kind, Message: asset.Kind + " 已达到只读扫描上限，完整备份前会重新计算"})
		}
	}
	return items
}
