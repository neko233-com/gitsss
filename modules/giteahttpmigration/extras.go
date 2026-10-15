// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
)

const maxCopiedBytes int64 = 512 << 20

// WikiPage is one wiki page. ContentBase64 is the Gitea wiki payload.
type WikiPage struct {
	Title         string `json:"title"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

// ReleaseAsset is a release file. Bytes are fetched from DownloadURL during apply.
type ReleaseAsset struct {
	ID          int64  `json:"id,omitempty"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"browser_download_url"`
}

// PullReview is one submitted review. Pending reviews are imported as comments.
type PullReview struct {
	ID       int64           `json:"id,omitempty"`
	Body     string          `json:"body,omitempty"`
	State    string          `json:"state,omitempty"`
	CommitID string          `json:"commit_id,omitempty"`
	User     *User           `json:"user,omitempty"`
	Comments []ReviewComment `json:"comments,omitempty"`
}

// ReviewComment is a line comment on a review.
// Resolver is set when the source comment was marked resolved.
type ReviewComment struct {
	ID       int64  `json:"id,omitempty"`
	Body     string `json:"body"`
	Path     string `json:"path,omitempty"`
	LineNum  uint64 `json:"position,omitempty"`
	User     *User  `json:"user,omitempty"`
	Resolver *User  `json:"resolver,omitempty"`
}

// PackageCopy is a package version linked to the repository.
// Only generic packages are copied byte for byte. Other package types use
// their own upload protocols and are reported, not re-encoded.
type PackageCopy struct {
	Type       string        `json:"type"`
	Name       string        `json:"name"`
	Version    string        `json:"version"`
	Repository *Repository   `json:"repository,omitempty"`
	Files      []PackageFile `json:"files,omitempty"`
}

// PackageFile is one file inside a package version.
type PackageFile struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256,omitempty"`
	CompositeKey string `json:"composite_key,omitempty"`
}

func (m *Migrator) discoverExtras(ctx context.Context, ref RepositoryRef, plan *RepositoryPlan) error {
	for i := range plan.Releases {
		if plan.Releases[i].ID == 0 {
			continue
		}
		endpoint := repoPath(ref) + "/releases/" + strconv.FormatInt(plan.Releases[i].ID, 10) + "/assets"
		if err := m.source.get(ctx, endpoint, &plan.Releases[i].Assets); err != nil {
			if isNotFound(err) {
				continue
			}
			return fmt.Errorf("list source release assets for %s/%s: %w", ref, plan.Releases[i].TagName, err)
		}
	}
	for i := range plan.Pulls {
		endpoint := repoPath(ref) + "/pulls/" + strconv.FormatInt(plan.Pulls[i].Number, 10) + "/reviews"
		if err := m.source.getAll(ctx, endpoint, &plan.Pulls[i].Reviews); err != nil {
			if isNotFound(err) {
				continue
			}
			return fmt.Errorf("list source reviews for %s#%d: %w", ref, plan.Pulls[i].Number, err)
		}
		for j := range plan.Pulls[i].Reviews {
			if plan.Pulls[i].Reviews[j].ID == 0 {
				continue
			}
			commentPath := endpoint + "/" + strconv.FormatInt(plan.Pulls[i].Reviews[j].ID, 10) + "/comments"
			if err := m.source.get(ctx, commentPath, &plan.Pulls[i].Reviews[j].Comments); err != nil && !isNotFound(err) {
				return fmt.Errorf("list source review comments for %s#%d: %w", ref, plan.Pulls[i].Number, err)
			}
		}
	}
	if err := m.source.getAll(ctx, repoPath(ref)+"/wiki/pages", &plan.Wiki); err != nil {
		if !isNotFound(err) {
			return fmt.Errorf("list source wiki pages for %s: %w", ref, err)
		}
	}
	for i := range plan.Wiki {
		if strings.TrimSpace(plan.Wiki[i].Title) == "" {
			return fmt.Errorf("source wiki page in %s has no title", ref)
		}
		var page WikiPage
		if err := m.source.get(ctx, repoPath(ref)+"/wiki/page/"+url.PathEscape(plan.Wiki[i].Title), &page); err != nil {
			return fmt.Errorf("read source wiki page %s/%s: %w", ref, plan.Wiki[i].Title, err)
		}
		plan.Wiki[i].ContentBase64 = page.ContentBase64
	}
	slices.SortFunc(plan.Wiki, func(a, b WikiPage) int { return cmp.Compare(a.Title, b.Title) })

	var listed []PackageCopy
	if err := m.source.getAll(ctx, "/packages/"+url.PathEscape(ref.Owner), &listed); err != nil {
		if !isNotFound(err) {
			return fmt.Errorf("list source packages for %s: %w", ref.Owner, err)
		}
	}
	for _, pkg := range listed {
		if pkg.Repository == nil || !strings.EqualFold(pkg.Repository.FullName, ref.String()) {
			continue
		}
		if !strings.EqualFold(pkg.Type, "container") {
			endpoint := "/packages/" + url.PathEscape(ref.Owner) + "/" + url.PathEscape(pkg.Type) + "/" + url.PathEscape(pkg.Name) + "/" + url.PathEscape(pkg.Version) + "/files"
			if err := m.source.get(ctx, endpoint, &pkg.Files); err != nil && !isNotFound(err) {
				return fmt.Errorf("list source package files %s/%s: %w", ref, pkg.Name, err)
			}
		}
		plan.Packages = append(plan.Packages, pkg)
	}
	slices.SortFunc(plan.Packages, func(a, b PackageCopy) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Version, b.Version))
	})
	return nil
}

func (m *Migrator) applyExtras(ctx context.Context, ref RepositoryRef, plan RepositoryPlan, stats *contentStats) error {
	for _, page := range plan.Wiki {
		payload := map[string]string{
			"title": page.Title, "content_base64": page.ContentBase64, "message": "import wiki page",
		}
		if err := m.target.post(ctx, repoPath(ref)+"/wiki/new", payload, nil); err != nil {
			return fmt.Errorf("create target wiki page %s/%s: %w", ref, page.Title, err)
		}
		stats.wikiPages++
	}
	linked := map[string]struct{}{}
	for _, pkg := range plan.Packages {
		copied, err := m.copyPackage(ctx, ref, pkg)
		if err != nil {
			return err
		}
		if !copied {
			stats.packagesSkipped++
			if !packageProtocolSupported(pkg.Type) {
				stats.notes = append(stats.notes, ref.String()+": package "+pkg.Type+"/"+pkg.Name+"@"+pkg.Version+" has no byte-for-byte protocol copier")
			}
			continue
		}
		key := strings.ToLower(pkg.Type + "/" + pkg.Name)
		if _, ok := linked[key]; !ok {
			endpoint := "/packages/" + url.PathEscape(ref.Owner) + "/" + url.PathEscape(pkg.Type) + "/" + url.PathEscape(pkg.Name) + "/-/link/" + url.PathEscape(ref.Name)
			if err := m.target.post(ctx, endpoint, nil, nil); err != nil && !isNotFound(err) {
				return fmt.Errorf("link target package %s/%s: %w", ref, pkg.Name, err)
			}
			linked[key] = struct{}{}
		}
		stats.packagesCopied++
	}
	return nil
}

func (m *Migrator) copyReleaseAssets(ctx context.Context, ref RepositoryRef, releaseID int64, assets []ReleaseAsset) (int, error) {
	if len(assets) == 0 {
		return 0, nil
	}
	if releaseID == 0 {
		return 0, fmt.Errorf("target release for %s has no id, so attachments cannot be uploaded", ref)
	}
	copied := 0
	for _, asset := range assets {
		if strings.TrimSpace(asset.Name) == "" || strings.TrimSpace(asset.DownloadURL) == "" {
			return copied, fmt.Errorf("source release asset in %s is missing a name or download URL", ref)
		}
		if asset.Size > maxCopiedBytes {
			return copied, fmt.Errorf("source release asset %s/%s is larger than %d bytes", ref, asset.Name, maxCopiedBytes)
		}
		file, size, err := m.source.downloadLimited(ctx, asset.DownloadURL, maxCopiedBytes)
		if err != nil {
			return copied, fmt.Errorf("download source release asset %s/%s: %w", ref, asset.Name, err)
		}
		err = m.target.uploadStream(ctx, http.MethodPost, m.target.apiURL+repoPath(ref)+"/releases/"+strconv.FormatInt(releaseID, 10)+"/assets?name="+url.QueryEscape(asset.Name), "application/octet-stream", file, size, http.StatusCreated)
		_ = os.Remove(file)
		if err != nil {
			return copied, fmt.Errorf("upload target release asset %s/%s: %w", ref, asset.Name, err)
		}
		copied++
	}
	return copied, nil
}

func (m *Migrator) copyGenericFile(ctx context.Context, ref RepositoryRef, pkg PackageCopy, file PackageFile) error {
	if strings.TrimSpace(file.Name) == "" {
		return fmt.Errorf("source package %s/%s has a file without a name", ref, pkg.Name)
	}
	if file.Size > maxCopiedBytes {
		return fmt.Errorf("source package file %s/%s is larger than %d bytes", ref, file.Name, maxCopiedBytes)
	}
	rawURL := strings.TrimSuffix(m.source.baseURL, "/") + "/api/packages/" + url.PathEscape(ref.Owner) + "/generic/" + url.PathEscape(pkg.Name) + "/" + url.PathEscape(pkg.Version) + "/" + url.PathEscape(file.Name)
	downloaded, size, err := m.source.downloadLimited(ctx, rawURL, maxCopiedBytes)
	if err != nil {
		return fmt.Errorf("download source package file %s/%s: %w", ref, file.Name, err)
	}
	defer os.Remove(downloaded)
	targetURL := strings.TrimSuffix(m.target.baseURL, "/") + "/api/packages/" + url.PathEscape(ref.Owner) + "/generic/" + url.PathEscape(pkg.Name) + "/" + url.PathEscape(pkg.Version) + "/" + url.PathEscape(file.Name)
	if err := m.target.uploadStream(ctx, http.MethodPut, targetURL, "application/octet-stream", downloaded, size, http.StatusCreated); err != nil {
		return fmt.Errorf("upload target package file %s/%s: %w", ref, file.Name, err)
	}
	return nil
}

func (m *Migrator) applyPullReviews(ctx context.Context, ref RepositoryRef, number int64, reviews []PullReview) (int, error) {
	copied := 0
	for _, review := range reviews {
		if err := m.createReview(ctx, ref, number, review); err != nil {
			return copied, err
		}
		copied++
	}
	return copied, nil
}

func (m *Migrator) createReview(ctx context.Context, ref RepositoryRef, number int64, review PullReview) error {
	event := reviewEvent(review.State)
	comments := make([]map[string]any, 0, len(review.Comments))
	for _, comment := range review.Comments {
		if strings.TrimSpace(comment.Path) == "" || strings.TrimSpace(comment.Body) == "" || comment.LineNum == 0 {
			continue
		}
		comments = append(comments, map[string]any{
			"path": comment.Path, "body": comment.Body, "new_position": comment.LineNum,
		})
	}
	payload := map[string]any{"event": event, "body": review.Body, "commit_id": review.CommitID}
	if len(comments) > 0 {
		payload["comments"] = comments
	}
	endpoint := repoPath(ref) + "/pulls/" + strconv.FormatInt(number, 10) + "/reviews"
	client := m.target
	if login := loginOf(review.User); login != "" {
		client = m.target.asUser(login)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	var output any
	if reviewHasResolver(review.Comments) {
		output = &created
	}
	err := client.do(ctx, http.MethodPost, endpoint, payload, output, http.StatusOK)
	if err == nil {
		return m.resolveReviewComments(ctx, ref, number, created.ID, review.Comments)
	}
	if len(comments) == 0 || !reviewRejected(err) {
		return fmt.Errorf("create target review on %s#%d: %w", ref, number, err)
	}
	if reviewHasResolver(review.Comments) {
		return fmt.Errorf("create target review on %s#%d: line comments were rejected, so the resolved state was not copied: %w", ref, number, err)
	}
	// The line no longer matches the mirrored diff. Keep the text on the review body.
	var b strings.Builder
	b.WriteString(review.Body)
	for _, comment := range review.Comments {
		if strings.TrimSpace(comment.Body) == "" {
			continue
		}
		b.WriteString("\n\n")
		if comment.Path != "" {
			b.WriteString(comment.Path)
			b.WriteString(": ")
		}
		b.WriteString(comment.Body)
	}
	fallback := map[string]any{"event": event, "body": strings.TrimSpace(b.String()), "commit_id": review.CommitID}
	if err := m.target.do(ctx, http.MethodPost, endpoint, fallback, nil, http.StatusOK); err != nil {
		return fmt.Errorf("create target review on %s#%d: %w", ref, number, err)
	}
	return nil
}

func reviewHasResolver(comments []ReviewComment) bool {
	for _, comment := range comments {
		if loginOf(comment.Resolver) != "" {
			return true
		}
	}
	return false
}

func (m *Migrator) resolveReviewComments(ctx context.Context, ref RepositoryRef, number, reviewID int64, source []ReviewComment) error {
	if !reviewHasResolver(source) {
		return nil
	}
	if reviewID == 0 {
		return fmt.Errorf("target review on %s#%d has no id, so resolved comments cannot be marked", ref, number)
	}
	var created []ReviewComment
	endpoint := repoPath(ref) + "/pulls/" + strconv.FormatInt(number, 10) + "/reviews/" + strconv.FormatInt(reviewID, 10) + "/comments"
	if err := m.target.get(ctx, endpoint, &created); err != nil {
		return fmt.Errorf("read target review comments on %s#%d: %w", ref, number, err)
	}
	used := map[int64]struct{}{}
	for _, comment := range source {
		if loginOf(comment.Resolver) == "" {
			continue
		}
		match := int64(0)
		for _, candidate := range created {
			if _, ok := used[candidate.ID]; ok {
				continue
			}
			if candidate.Path == comment.Path && candidate.Body == comment.Body {
				match = candidate.ID
				break
			}
		}
		if match == 0 {
			return fmt.Errorf("resolved comment %s on %s#%d was not found on the target", comment.Path, ref, number)
		}
		used[match] = struct{}{}
		resolvePath := repoPath(ref) + "/pulls/comments/" + strconv.FormatInt(match, 10) + "/resolve"
		if err := m.target.noContent(ctx, http.MethodPost, resolvePath, nil); err != nil {
			return fmt.Errorf("resolve target comment %s on %s#%d: %w", comment.Path, ref, number, err)
		}
	}
	return nil
}

func reviewEvent(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "APPROVED", "REQUEST_CHANGES", "COMMENT":
		return strings.ToUpper(strings.TrimSpace(state))
	default:
		return "COMMENT"
	}
}

func reviewRejected(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusUnprocessableEntity || apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusNotFound
}

func isNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func sameOrigin(baseURL, rawURL string, allowQuery bool) error {
	if !allowQuery {
		return requireSameOrigin(baseURL, rawURL)
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if target.Scheme != "http" && target.Scheme != "https" || target.User != nil || target.Fragment != "" {
		return errors.New("must be an HTTP(S) URL without embedded credentials or fragment")
	}
	if !strings.EqualFold(base.Scheme, target.Scheme) || !strings.EqualFold(base.Host, target.Host) {
		return errors.New("must use the source server origin")
	}
	return nil
}

type downloadedFile struct {
	Path        string
	Size        int64
	ContentType string
}

func (c *apiClient) downloadLimited(ctx context.Context, rawURL string, limit int64) (string, int64, error) {
	file, err := c.downloadFile(ctx, rawURL, limit, nil)
	if err != nil {
		return "", 0, err
	}
	return file.Path, file.Size, nil
}

func (c *apiClient) downloadFile(ctx context.Context, rawURL string, limit int64, headers map[string]string) (downloadedFile, error) {
	if err := requireSameOrigin(c.baseURL, rawURL); err != nil {
		return downloadedFile{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return downloadedFile{}, err
	}
	c.authorize(req)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return downloadedFile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return downloadedFile{}, c.redact(&APIError{Method: http.MethodGet, URL: rawURL, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(message))})
	}
	file, err := os.CreateTemp("", "gitsss-import-*")
	if err != nil {
		return downloadedFile{}, err
	}
	copied, err := io.Copy(file, io.LimitReader(resp.Body, limit+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(file.Name())
		return downloadedFile{}, err
	}
	if copied > limit {
		_ = os.Remove(file.Name())
		return downloadedFile{}, fmt.Errorf("download exceeded %d bytes", limit)
	}
	return downloadedFile{Path: file.Name(), Size: copied, ContentType: resp.Header.Get("Content-Type")}, nil
}

func (c *apiClient) uploadStream(ctx context.Context, method, rawURL, contentType, filePath string, size, expected int64) error {
	if err := sameOrigin(c.baseURL, rawURL, true); err != nil {
		return err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	req, err := http.NewRequestWithContext(ctx, method, rawURL, file)
	if err != nil {
		return err
	}
	req.ContentLength = size
	c.authorize(req)
	req.Header.Set("Content-Type", contentType)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if int64(resp.StatusCode) != expected {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return c.redact(&APIError{Method: method, URL: rawURL, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(message))})
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *apiClient) authorize(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	if c.basic {
		req.SetBasicAuth(c.user, c.secret)
	} else {
		req.Header.Set("Authorization", "token "+c.secret)
	}
	if c.sudo != "" {
		req.Header.Set("Sudo", c.sudo)
	}
}

func (c *apiClient) redact(err *APIError) error {
	for _, secret := range c.secrets {
		if secret != "" {
			err.Body = strings.ReplaceAll(err.Body, secret, "[REDACTED]")
			err.URL = strings.ReplaceAll(err.URL, secret, "[REDACTED]")
		}
	}
	return err
}
