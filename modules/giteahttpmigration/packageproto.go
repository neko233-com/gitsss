// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/neko233-com/gitsss/modules/json"
)

func packageProtocolSupported(packageType string) bool {
	switch strings.ToLower(packageType) {
	case "generic", "npm", "maven", "pypi", "container":
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
	case "npm":
		return true, m.copyNpmPackage(ctx, ref, pkg)
	case "maven":
		return true, m.copyMavenPackage(ctx, ref, pkg)
	case "pypi":
		return true, m.copyPyPIPackage(ctx, ref, pkg)
	case "container":
		return true, m.copyContainerPackage(ctx, ref, pkg)
	default:
		return false, nil
	}
}

func npmPackagePath(owner, name, version, filename string) (string, error) {
	if owner == "" || name == "" || version == "" || filename == "" {
		return "", fmt.Errorf("npm package %s is missing owner, name, version, or file", name)
	}
	if scope, pkg, ok := strings.Cut(name, "/"); ok && strings.HasPrefix(scope, "@") && pkg != "" {
		return "/api/packages/" + url.PathEscape(owner) + "/npm/@" + url.PathEscape(strings.TrimPrefix(scope, "@")) + "/" + url.PathEscape(pkg) + "/-/" + url.PathEscape(version) + "/" + url.PathEscape(filename), nil
	}
	if strings.Contains(name, "/") {
		return "", fmt.Errorf("npm package name %q is not a scoped name", name)
	}
	return "/api/packages/" + url.PathEscape(owner) + "/npm/" + url.PathEscape(name) + "/-/" + url.PathEscape(version) + "/" + url.PathEscape(filename), nil
}

func npmPublishDocument(name, version, filename string, data []byte) ([]byte, error) {
	if name == "" || version == "" || filename == "" {
		return nil, errors.New("npm publish document needs a name, version, and file")
	}
	sum := sha512.Sum512(data)
	document := map[string]any{
		"name":      name,
		"dist-tags": map[string]string{"latest": version},
		"versions": map[string]any{
			version: map[string]any{
				"name":    name,
				"version": version,
				"dist": map[string]string{
					"integrity": "sha512-" + base64.StdEncoding.EncodeToString(sum[:]),
					"tarball":   filename,
				},
			},
		},
		"_attachments": map[string]any{
			filename: map[string]any{
				"content_type": "application/octet-stream",
				"data":         base64.StdEncoding.EncodeToString(data),
				"length":       len(data),
			},
		},
	}
	return json.Marshal(document)
}

func mavenFilePath(owner, packageName, version, filename string) (string, error) {
	group, artifact, ok := strings.Cut(packageName, ":")
	if !ok || group == "" || artifact == "" || version == "" || filename == "" || strings.Contains(filename, "/") {
		return "", fmt.Errorf("maven package %q needs group:artifact, version, and a file name", packageName)
	}
	var segments []string
	for part := range strings.SplitSeq(group, ".") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("maven group %q is invalid", group)
		}
		segments = append(segments, url.PathEscape(part))
	}
	segments = append(segments, url.PathEscape(artifact), url.PathEscape(version), url.PathEscape(filename))
	return "/api/packages/" + url.PathEscape(owner) + "/maven/" + strings.Join(segments, "/"), nil
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

func (m *Migrator) copyNpmPackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) error {
	if len(pkg.Files) == 0 {
		return fmt.Errorf("npm package %s/%s has no files", ref, pkg.Name)
	}
	file := pkg.Files[0]
	downloadPath, err := npmPackagePath(ref.Owner, pkg.Name, pkg.Version, file.Name)
	if err != nil {
		return err
	}
	rawURL := strings.TrimSuffix(m.source.baseURL, "/") + downloadPath
	downloaded, err := m.source.downloadFile(ctx, rawURL, maxCopiedBytes, nil)
	if err != nil {
		return fmt.Errorf("download npm package %s/%s: %w", ref, pkg.Name, err)
	}
	defer os.Remove(downloaded.Path)
	data, err := os.ReadFile(downloaded.Path)
	if err != nil {
		return err
	}
	document, err := npmPublishDocument(pkg.Name, pkg.Version, file.Name, data)
	if err != nil {
		return err
	}
	publishPath, err := npmPackagePath(ref.Owner, pkg.Name, pkg.Version, file.Name)
	if err != nil {
		return err
	}
	publishPath = strings.TrimSuffix(publishPath, "/-/"+url.PathEscape(pkg.Version)+"/"+url.PathEscape(file.Name))
	temp, err := os.CreateTemp("", "gitsss-npm-*")
	if err != nil {
		return err
	}
	if _, err := temp.Write(document); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return err
	}
	_ = temp.Close()
	defer os.Remove(temp.Name())
	targetURL := strings.TrimSuffix(m.target.baseURL, "/") + publishPath
	if err := m.target.uploadStream(ctx, http.MethodPut, targetURL, "application/json", temp.Name(), int64(len(document)), http.StatusCreated); err != nil {
		return fmt.Errorf("publish npm package %s/%s: %w", ref, pkg.Name, err)
	}
	return nil
}

func (m *Migrator) copyMavenPackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) error {
	if len(pkg.Files) == 0 {
		return fmt.Errorf("maven package %s/%s has no files", ref, pkg.Name)
	}
	for _, file := range pkg.Files {
		filePath, err := mavenFilePath(ref.Owner, pkg.Name, pkg.Version, file.Name)
		if err != nil {
			return err
		}
		if err := m.copySamePath(ctx, ref, pkg, filePath); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) copyPyPIPackage(ctx context.Context, ref RepositoryRef, pkg PackageCopy) error {
	if len(pkg.Files) == 0 {
		return fmt.Errorf("pypi package %s/%s has no files", ref, pkg.Name)
	}
	for _, file := range pkg.Files {
		if file.SHA256 == "" {
			return fmt.Errorf("pypi file %s/%s has no sha256", ref, file.Name)
		}
		downloadPath := "/api/packages/" + url.PathEscape(ref.Owner) + "/pypi/files/" + url.PathEscape(pkg.Name) + "/" + url.PathEscape(pkg.Version) + "/" + url.PathEscape(file.Name)
		downloaded, err := m.source.downloadFile(ctx, strings.TrimSuffix(m.source.baseURL, "/")+downloadPath, maxCopiedBytes, nil)
		if err != nil {
			return fmt.Errorf("download pypi package %s/%s: %w", ref, file.Name, err)
		}
		bodyPath, size, contentType, err := pypiMultipart(pkg.Name, pkg.Version, file.Name, file.SHA256, downloaded.Path)
		_ = os.Remove(downloaded.Path)
		if err != nil {
			return err
		}
		targetURL := strings.TrimSuffix(m.target.baseURL, "/") + "/api/packages/" + url.PathEscape(ref.Owner) + "/pypi/"
		err = m.target.uploadStream(ctx, http.MethodPost, targetURL, contentType, bodyPath, size, http.StatusCreated)
		_ = os.Remove(bodyPath)
		if err != nil {
			return fmt.Errorf("upload pypi package %s/%s: %w", ref, file.Name, err)
		}
	}
	return nil
}

func pypiMultipart(name, version, filename, sha256sum, dataPath string) (string, int64, string, error) {
	data, err := os.Open(dataPath)
	if err != nil {
		return "", 0, "", err
	}
	defer data.Close()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("name", name); err != nil {
		return "", 0, "", err
	}
	if err := writer.WriteField("version", version); err != nil {
		return "", 0, "", err
	}
	if err := writer.WriteField("sha256_digest", sha256sum); err != nil {
		return "", 0, "", err
	}
	part, err := writer.CreateFormFile("content", filename)
	if err != nil {
		return "", 0, "", err
	}
	if _, err := io.Copy(part, data); err != nil {
		return "", 0, "", err
	}
	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		return "", 0, "", err
	}
	file, err := os.CreateTemp("", "gitsss-pypi-*")
	if err != nil {
		return "", 0, "", err
	}
	if _, err := file.Write(body.Bytes()); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", 0, "", err
	}
	_ = file.Close()
	return file.Name(), int64(body.Len()), contentType, nil
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
	if err := m.target.uploadStream(ctx, http.MethodPut, targetURL, contentType, manifest.Path, manifest.Size, http.StatusCreated); err != nil {
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
	if err := m.target.uploadStream(ctx, http.MethodPost, targetURL, "application/octet-stream", blob.Path, blob.Size, http.StatusCreated); err != nil {
		return fmt.Errorf("upload container blob %s: %w", digest, err)
	}
	return nil
}

func (m *Migrator) copySamePath(ctx context.Context, ref RepositoryRef, pkg PackageCopy, filePath string) error {
	sourceURL := strings.TrimSuffix(m.source.baseURL, "/") + filePath
	downloaded, err := m.source.downloadFile(ctx, sourceURL, maxCopiedBytes, nil)
	if err != nil {
		return fmt.Errorf("download %s package %s: %w", pkg.Type, ref, err)
	}
	defer os.Remove(downloaded.Path)
	targetURL := strings.TrimSuffix(m.target.baseURL, "/") + filePath
	if err := m.target.uploadStream(ctx, http.MethodPut, targetURL, "application/octet-stream", downloaded.Path, downloaded.Size, http.StatusCreated); err != nil {
		return fmt.Errorf("upload %s package %s: %w", pkg.Type, ref, err)
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
