// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packagemirror

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	std_html "html"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/neko233-com/gitsss/modules/json"

	"golang.org/x/net/html"
)

const maxMetadataSize = 32 * 1024 * 1024

type Artifact struct {
	Package string
	URL     string
}

func (c *Cache) sign(kind, owner string, artifact Artifact) string {
	data, _ := json.Marshal(artifact)
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, []byte(c.cfg.Secret))
	_, _ = fmt.Fprintf(mac, "%s\x00%s\x00%s", kind, owner, payload)
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Cache) ResolveArtifact(kind, owner, token string) (Artifact, error) {
	var artifact Artifact
	payload, signature, ok := strings.Cut(token, ".")
	if !ok || len(token) > 8192 {
		return artifact, &Error{http.StatusBadRequest}
	}
	mac := hmac.New(sha256.New, []byte(c.cfg.Secret))
	_, _ = fmt.Fprintf(mac, "%s\x00%s\x00%s", kind, owner, payload)
	provided, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(provided, mac.Sum(nil)) {
		return artifact, &Error{http.StatusBadRequest}
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || json.Unmarshal(data, &artifact) != nil || artifact.Package == "" {
		return artifact, &Error{http.StatusBadRequest}
	}
	u, err := url.Parse(artifact.URL)
	if err != nil || !c.allowed(kind, u) {
		return artifact, &Error{http.StatusBadRequest}
	}
	return artifact, nil
}

func (c *Cache) artifactLink(kind, owner, packageName, registry, raw string, base *url.URL) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u = base.ResolveReference(u)
	fragment := u.Fragment
	u.Fragment = ""
	if !c.allowed(kind, u) {
		return "", errors.New("package mirror artifact host refused")
	}
	filename := path.Base(u.Path)
	if filename == "." || filename == "/" || filename == "" {
		return "", errors.New("package mirror artifact filename missing")
	}
	prefix := "/mirror/"
	if kind == "npm" {
		prefix = "/-/mirror/"
	}
	link := registry + prefix + c.sign(kind, owner, Artifact{Package: packageName, URL: u.String()}) + "/" + url.PathEscape(filename)
	if fragment != "" {
		link += "#" + fragment
	}
	return link, nil
}

func metadataBytes(entry *Entry) ([]byte, error) {
	if entry.Size > maxMetadataSize {
		return nil, errors.New("package mirror metadata too large")
	}
	f, reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(reader)
}

// NpmMetadata keeps integrity and dependency fields while routing tarballs through this cache.
func (c *Cache) NpmMetadata(entry *Entry, owner, packageName, registry string) ([]byte, error) {
	data, err := metadataBytes(entry)
	if err != nil {
		return nil, err
	}
	var metadata map[string]any
	if err = json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	versions, ok := metadata["versions"].(map[string]any)
	if !ok {
		return nil, errors.New("package mirror npm versions missing")
	}
	base, _ := url.Parse(c.cfg.Upstreams["npm"] + "/")
	for _, value := range versions {
		version, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("package mirror npm version invalid")
		}
		dist, ok := version["dist"].(map[string]any)
		if !ok {
			return nil, errors.New("package mirror npm distribution missing")
		}
		raw, ok := dist["tarball"].(string)
		if !ok {
			return nil, errors.New("package mirror npm tarball missing")
		}
		link, err := c.artifactLink("npm", owner, packageName, registry, raw, base)
		if err != nil {
			return nil, err
		}
		dist["tarball"] = link
	}
	return json.Marshal(metadata)
}

// ComposerMetadata preserves version metadata and routes approved distribution archives through the cache.
func (c *Cache) ComposerMetadata(entry *Entry, owner, packageName, registry string) ([]byte, error) {
	data, err := metadataBytes(entry)
	if err != nil {
		return nil, err
	}
	var metadata map[string]any
	if err = json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	packages, ok := metadata["packages"].(map[string]any)
	if !ok {
		return nil, errors.New("package mirror composer packages missing")
	}
	versions, ok := packages[packageName].([]any)
	if !ok {
		return nil, errors.New("package mirror composer package missing")
	}
	base, _ := url.Parse(c.cfg.Upstreams["composer"] + "/")
	for _, value := range versions {
		version, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("package mirror composer version invalid")
		}
		dist, ok := version["dist"].(map[string]any)
		if !ok {
			continue
		}
		raw, ok := dist["url"].(string)
		if !ok || raw == "" {
			continue
		}
		link, err := c.artifactLink("composer", owner, packageName, registry, raw, base)
		if err != nil {
			return nil, err
		}
		dist["url"] = link
	}
	return json.Marshal(metadata)
}

// PyPIMetadata emits only Simple API links, retaining hashes, Python constraints and yanked markers.
func (c *Cache) PyPIMetadata(entry *Entry, owner, packageName, registry, relative string) ([]byte, error) {
	data, err := metadataBytes(entry)
	if err != nil {
		return nil, err
	}
	raw, err := c.UpstreamURL("pypi", relative)
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(raw)
	var output strings.Builder
	output.WriteString("<!DOCTYPE html><html><head><meta name=\"pypi:repository-version\" content=\"1.0\"></head><body>\n")
	tokenizer := html.NewTokenizer(strings.NewReader(string(data)))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			if !errors.Is(tokenizer.Err(), io.EOF) {
				return nil, tokenizer.Err()
			}
			break
		}
		if tokenType != html.StartTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data != "a" {
			continue
		}
		var href string
		for _, attr := range token.Attr {
			if attr.Key == "href" {
				href = attr.Val
			}
		}
		if href == "" {
			continue
		}
		link, err := c.artifactLink("pypi", owner, packageName, registry, href, base)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "<a href=\"%s\"", std_html.EscapeString(link))
		for _, attr := range token.Attr {
			switch attr.Key {
			case "data-requires-python", "data-yanked", "data-core-metadata", "data-dist-info-metadata":
				fmt.Fprintf(&output, " %s=\"%s\"", attr.Key, std_html.EscapeString(attr.Val))
			}
		}
		u, _ := url.Parse(href)
		fmt.Fprintf(&output, ">%s</a>\n", std_html.EscapeString(path.Base(u.Path)))
	}
	output.WriteString("</body></html>\n")
	return []byte(output.String()), nil
}
