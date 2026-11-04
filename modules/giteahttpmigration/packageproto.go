// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/neko233-com/gitsss/modules/json"
)

func packageProtocolSupported(packageType string) bool {
	switch strings.ToLower(packageType) {
	case "generic", "container", "helm", "debian", "alpine", "rubygems":
		return true
	default:
		return false
	}
}

func (m *Migrator) copyPackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) (bool, error) {
	switch strings.ToLower(pkg.Type) {
	case "generic":
		if len(pkg.Files) == 0 {
			return false, nil
		}
		for _, file := range pkg.Files {
			if err := m.copyGenericFile(ctx, ref, pkg, file); err != nil {
				return false, err
			}
		}
		return true, nil
	case "container":
		return true, m.copyContainerPackage(ctx, ref, pkg)
	case "helm":
		return true, m.copyHelmPackage(ctx, ref, pkg)
	case "debian":
		return true, m.copyDebianPackage(ctx, ref, pkg)
	case "alpine":
		return true, m.copyAlpinePackage(ctx, ref, pkg)
	case "rubygems":
		return true, m.copyRubyGemsPackage(ctx, ref, pkg)
	default:
		return false, nil
	}
}

func containerDigests(manifest []byte) ([]string, error) {
	var document struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(manifest, &document); err != nil {
		return nil, fmt.Errorf("read container manifest: %w", err)
	}
	seen := map[string]struct{}{}
	var digests []string
	add := func(digest string) {
		if digest == "" {
			return
		}
		if _, ok := seen[digest]; ok {
			return
		}
		seen[digest] = struct{}{}
		digests = append(digests, digest)
	}
	add(document.Config.Digest)
	for _, layer := range document.Layers {
		add(layer.Digest)
	}
	for _, nested := range document.Manifests {
		add(nested.Digest)
	}
	return digests, nil
}

func (m *Migrator) copyContainerPackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) error {
	owner := strings.ToLower(ref.Owner)
	image := pkg.Name
	tag := pkg.Version
	manifestURL := strings.TrimSuffix(m.source.baseURL, "/") + "/v2/" + url.PathEscape(owner) + "/" + escapeImage(image) + "/manifests/" + url.PathEscape(tag)
	manifest, err := m.source.downloadFile(ctx, manifestURL, maxCopiedBytes, map[string]string{
		"Accept": "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.index.v1+json",
	})
	if err != nil {
		return fmt.Errorf("download container manifest %s/%s: %w", ref, image, err)
	}
	defer os.Remove(manifest.Path)
	raw, err := os.ReadFile(manifest.Path)
	if err != nil {
		return err
	}
	digests, err := containerDigests(raw)
	if err != nil {
		return err
	}
	for _, digest := range digests {
		if err := m.copyContainerBlob(ctx, ref, owner, image, digest); err != nil {
			return err
		}
	}
	contentType := manifest.ContentType
	if contentType == "" {
		contentType = "application/vnd.docker.distribution.manifest.v2+json"
	}
	targetURL := strings.TrimSuffix(m.target.baseURL, "/") + "/v2/" + url.PathEscape(owner) + "/" + escapeImage(image) + "/manifests/" + url.PathEscape(tag)
	if err := m.target.uploadStream(ctx, http.MethodPut, targetURL, contentType, manifest.Path, manifest.Size); err != nil {
		return fmt.Errorf("upload container manifest %s/%s: %w", ref, image, err)
	}
	return nil
}

func (m *Migrator) copyContainerBlob(ctx context.Context, ref RepositoryRef, owner, image, digest string) error {
	blobURL := strings.TrimSuffix(m.source.baseURL, "/") + "/v2/" + url.PathEscape(owner) + "/" + escapeImage(image) + "/blobs/" + url.PathEscape(digest)
	blob, err := m.source.downloadFile(ctx, blobURL, maxCopiedBytes, nil)
	if err != nil {
		return fmt.Errorf("download container blob %s %s: %w", ref, digest, err)
	}
	defer os.Remove(blob.Path)
	sum, err := fileSHA256(blob.Path)
	if err != nil {
		return err
	}
	if "sha256:"+sum != digest {
		return fmt.Errorf("container blob %s digest is %s, not %s", digest, "sha256:"+sum, digest)
	}
	targetURL := strings.TrimSuffix(m.target.baseURL, "/") + "/v2/" + url.PathEscape(owner) + "/" + escapeImage(image) + "/blobs/uploads/?digest=" + url.QueryEscape(digest)
	if err := m.target.uploadStream(ctx, http.MethodPost, targetURL, "application/octet-stream", blob.Path, blob.Size); err != nil {
		return fmt.Errorf("upload container blob %s: %w", digest, err)
	}
	return nil
}

func escapeImage(image string) string {
	parts := strings.Split(image, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func (m *Migrator) copyHelmPackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) error {
	filename := helmChartFilename(pkg)
	download := "/api/packages/" + url.PathEscape(ref.Owner) + "/helm/" + url.PathEscape(filename)
	downloaded, err := m.source.downloadFile(ctx, strings.TrimSuffix(m.source.baseURL, "/")+download, maxCopiedBytes, nil)
	if err != nil {
		return fmt.Errorf("download helm chart %s/%s: %w", ref, pkg.Name, err)
	}
	defer os.Remove(downloaded.Path)
	targetURL := strings.TrimSuffix(m.target.baseURL, "/") + "/api/packages/" + url.PathEscape(ref.Owner) + "/helm/api/charts"
	if err := m.target.uploadStream(ctx, http.MethodPost, targetURL, "application/gzip", downloaded.Path, downloaded.Size); err != nil {
		return fmt.Errorf("upload helm chart %s/%s: %w", ref, pkg.Name, err)
	}
	return nil
}

func helmChartFilename(pkg PackageCopy) string {
	if len(pkg.Files) > 0 && pkg.Files[0].Name != "" {
		return pkg.Files[0].Name
	}
	return strings.ToLower(pkg.Name + "-" + pkg.Version + ".tgz")
}

func debianCoordinates(pkg PackageCopy, file PackageFile) (string, string, string, error) {
	distribution, component, ok := strings.Cut(file.CompositeKey, "|")
	if !ok || distribution == "" || component == "" || strings.Contains(component, "|") {
		return "", "", "", fmt.Errorf("debian package %s is missing distribution|component", pkg.Name)
	}
	suffix := "_" + pkg.Version + "_"
	name, arch, found := strings.Cut(strings.TrimSuffix(file.Name, ".deb"), suffix)
	if !found || name != pkg.Name || arch == "" {
		return "", "", "", fmt.Errorf("debian file %q does not match %s_%s_<arch>.deb", file.Name, pkg.Name, pkg.Version)
	}
	return distribution, component, arch, nil
}

func (m *Migrator) copyDebianPackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) error {
	if len(pkg.Files) == 0 {
		return fmt.Errorf("debian package %s/%s has no files", ref, pkg.Name)
	}
	for _, file := range pkg.Files {
		distribution, component, arch, err := debianCoordinates(pkg, file)
		if err != nil {
			return err
		}
		base := "/api/packages/" + url.PathEscape(ref.Owner) + "/debian/pool/" + url.PathEscape(distribution) + "/" + url.PathEscape(component)
		download := base + "/" + url.PathEscape(pkg.Name+"_"+pkg.Version+"_"+arch+".deb")
		downloaded, err := m.source.downloadFile(ctx, strings.TrimSuffix(m.source.baseURL, "/")+download, maxCopiedBytes, nil)
		if err != nil {
			return fmt.Errorf("download debian package %s/%s: %w", ref, pkg.Name, err)
		}
		targetURL := strings.TrimSuffix(m.target.baseURL, "/") + base + "/upload"
		err = m.target.uploadStream(ctx, http.MethodPut, targetURL, "application/vnd.debian.binary-package", downloaded.Path, downloaded.Size)
		_ = os.Remove(downloaded.Path)
		if err != nil {
			return fmt.Errorf("upload debian package %s/%s: %w", ref, pkg.Name, err)
		}
	}
	return nil
}

func alpineCoordinates(file PackageFile) (string, string, string, error) {
	parts := strings.Split(file.CompositeKey, "|")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("alpine file %s is missing branch|repository|architecture", file.Name)
	}
	return parts[0], parts[1], parts[2], nil
}

func (m *Migrator) copyAlpinePackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) error {
	if len(pkg.Files) == 0 {
		return fmt.Errorf("alpine package %s/%s has no files", ref, pkg.Name)
	}
	for _, file := range pkg.Files {
		branch, repository, arch, err := alpineCoordinates(file)
		if err != nil {
			return err
		}
		download := "/api/packages/" + url.PathEscape(ref.Owner) + "/alpine/" + url.PathEscape(branch) + "/" + url.PathEscape(repository) + "/" + url.PathEscape(arch) + "/" + url.PathEscape(file.Name)
		downloaded, err := m.source.downloadFile(ctx, strings.TrimSuffix(m.source.baseURL, "/")+download, maxCopiedBytes, nil)
		if err != nil {
			return fmt.Errorf("download alpine package %s/%s: %w", ref, file.Name, err)
		}
		targetURL := strings.TrimSuffix(m.target.baseURL, "/") + "/api/packages/" + url.PathEscape(ref.Owner) + "/alpine/" + url.PathEscape(branch) + "/" + url.PathEscape(repository)
		err = m.target.uploadStream(ctx, http.MethodPut, targetURL, "application/vnd.alpine.apk", downloaded.Path, downloaded.Size)
		_ = os.Remove(downloaded.Path)
		if err != nil {
			return fmt.Errorf("upload alpine package %s/%s: %w", ref, file.Name, err)
		}
	}
	return nil
}

func (m *Migrator) copyRubyGemsPackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) error {
	if len(pkg.Files) == 0 {
		return fmt.Errorf("rubygems package %s/%s has no files", ref, pkg.Name)
	}
	file := pkg.Files[0]
	download := "/api/packages/" + url.PathEscape(ref.Owner) + "/rubygems/gems/" + url.PathEscape(file.Name)
	downloaded, err := m.source.downloadFile(ctx, strings.TrimSuffix(m.source.baseURL, "/")+download, maxCopiedBytes, nil)
	if err != nil {
		return fmt.Errorf("download rubygems package %s/%s: %w", ref, pkg.Name, err)
	}
	defer os.Remove(downloaded.Path)
	targetURL := strings.TrimSuffix(m.target.baseURL, "/") + "/api/packages/" + url.PathEscape(ref.Owner) + "/rubygems/api/v1/gems"
	if err := m.target.uploadStream(ctx, http.MethodPost, targetURL, "application/octet-stream", downloaded.Path, downloaded.Size); err != nil {
		return fmt.Errorf("upload rubygems package %s/%s: %w", ref, pkg.Name, err)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
