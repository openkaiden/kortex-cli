/**********************************************************************
 * Copyright (C) 2026 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 **********************************************************************/

package features

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ociFeature implements Feature for OCI registry references.
type ociFeature struct {
	id     string
	client *http.Client
}

var _ Feature = (*ociFeature)(nil)

// NewOCIFeatureWithClient returns an OCI Feature that uses the provided HTTP client.
// Intended for testing; production code uses the Feature instances returned by FromMap.
func NewOCIFeatureWithClient(id string, client *http.Client) Feature {
	return &ociFeature{id: id, client: client}
}

func (f *ociFeature) ID() string { return f.id }

func (f *ociFeature) httpClient() *http.Client {
	if f.client != nil {
		return f.client
	}
	return http.DefaultClient
}

func (f *ociFeature) Download(ctx context.Context, destDir string) (FeatureMetadata, error) {
	registry, repository, ref, err := parseOCIRef(f.id)
	if err != nil {
		return nil, fmt.Errorf("parsing OCI reference %q: %w", f.id, err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}

	manifest, token, err := f.fetchManifest(ctx, registry, repository, ref)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}

	for _, layer := range manifest.Layers {
		if err := f.downloadAndExtractLayer(ctx, registry, repository, layer.Digest, token, destDir); err != nil {
			return nil, fmt.Errorf("extracting layer %s: %w", layer.Digest, err)
		}
	}

	return parseMetadata(destDir)
}

// parseOCIRef parses an OCI reference into registry, repository, and ref (tag or digest).
// Format: [registry/]repository[:tag|@digest]
func parseOCIRef(id string) (registry, repository, ref string, err error) {
	// Split off digest.
	if i := strings.Index(id, "@"); i >= 0 {
		ref = id[i+1:]
		id = id[:i]
	} else {
		// Split off tag: last colon that appears after the first slash.
		firstSlash := strings.Index(id, "/")
		lastColon := strings.LastIndex(id, ":")
		if lastColon > firstSlash {
			ref = id[lastColon+1:]
			id = id[:lastColon]
		}
	}
	if ref == "" {
		ref = "latest"
	}

	// Determine registry vs. repository.
	// The first component is a registry if it contains '.' or ':' or equals "localhost".
	parts := strings.SplitN(id, "/", 2)
	if len(parts) == 2 {
		first := parts[0]
		if strings.ContainsAny(first, ".:") || first == "localhost" {
			return first, parts[1], ref, nil
		}
	}
	// No explicit registry; default to ghcr.io.
	return "ghcr.io", id, ref, nil
}

type ociManifest struct {
	Layers []ociDescriptor `json:"layers"`
}

type ociDescriptor struct {
	Digest string `json:"digest"`
}

// fetchManifest fetches the OCI manifest, handling anonymous Bearer auth automatically.
// Returns the manifest, the bearer token (may be empty), and any error.
func (f *ociFeature) fetchManifest(ctx context.Context, registry, repository, ref string) (*ociManifest, string, error) {
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repository, ref)
	client := f.httpClient()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}

	var token string
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		token, err = fetchBearerToken(ctx, client, wwwAuth, repository)
		if err != nil {
			return nil, "", fmt.Errorf("fetching bearer token: %w", err)
		}

		req, err = http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err = client.Do(req)
		if err != nil {
			return nil, "", err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("manifest fetch failed: %s", resp.Status)
	}

	var manifest ociManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, "", fmt.Errorf("decoding manifest: %w", err)
	}

	return &manifest, token, nil
}

// fetchBearerToken fetches an anonymous Bearer token using the WWW-Authenticate challenge.
func fetchBearerToken(ctx context.Context, client *http.Client, wwwAuth, repository string) (string, error) {
	realm, service, scope := parseWWWAuthenticate(wwwAuth, repository)
	if realm == "" {
		return "", fmt.Errorf("no realm in WWW-Authenticate: %q", wwwAuth)
	}

	tokenURL, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("invalid realm URL: %w", err)
	}

	q := tokenURL.Query()
	if service != "" {
		q.Set("service", service)
	}
	if scope != "" {
		q.Set("scope", scope)
	}
	tokenURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token fetch failed: %s", resp.Status)
	}

	var tokenData struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	if tokenData.Token != "" {
		return tokenData.Token, nil
	}
	return tokenData.AccessToken, nil
}

// parseWWWAuthenticate parses a Bearer WWW-Authenticate header into realm, service, scope.
func parseWWWAuthenticate(header, repository string) (realm, service, scope string) {
	header = strings.TrimPrefix(header, "Bearer ")

	params := make(map[string]string)
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			v := strings.Trim(strings.TrimSpace(kv[1]), `"`)
			params[k] = v
		}
	}

	realm = params["realm"]
	service = params["service"]
	scope = params["scope"]
	if scope == "" {
		scope = fmt.Sprintf("repository:%s:pull", repository)
	}
	return
}

// downloadAndExtractLayer downloads a blob by digest and extracts it into destDir.
func (f *ociFeature) downloadAndExtractLayer(ctx context.Context, registry, repository, digest, token, destDir string) error {
	blobURL := fmt.Sprintf("https://%s/v2/%s/blobs/%s", registry, repository, digest)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := f.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("blob fetch failed: %s", resp.Status)
	}

	return extractTar(resp.Body, destDir)
}

// extractTar extracts a tar stream (auto-detecting gzip compression) into destDir.
// Peeks at the first two bytes: 0x1f 0x8b indicates gzip; otherwise plain tar.
func extractTar(r io.Reader, destDir string) error {
	peek := make([]byte, 2)
	n, err := io.ReadFull(r, peek)
	if err != nil && err != io.ErrUnexpectedEOF {
		if n == 0 {
			return nil // empty stream
		}
		return err
	}

	combined := io.MultiReader(bytes.NewReader(peek[:n]), r)

	var tr *tar.Reader
	if n >= 2 && peek[0] == 0x1f && peek[1] == 0x8b {
		gr, err := gzip.NewReader(combined)
		if err != nil {
			return fmt.Errorf("creating gzip reader: %w", err)
		}
		defer gr.Close()
		tr = tar.NewReader(gr)
	} else {
		tr = tar.NewReader(combined)
	}

	return extractTarEntries(tr, destDir)
}

// safeTarTarget resolves name relative to destDir and verifies the result
// stays within destDir, preventing directory-traversal attacks.
func safeTarTarget(destDir, name string) (string, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(name))
	if cleanPath == "." || filepath.IsAbs(cleanPath) ||
		cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path in tar archive: %s", name)
	}

	target := filepath.Join(destDir, cleanPath)
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absDest, absTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid path in tar archive: %s", name)
	}
	return target, nil
}

// extractTarEntries extracts all entries from a tar reader into destDir,
// sanitizing paths to prevent directory traversal. Symlinks and hard links
// are rejected to avoid escaping destDir via link chains.
func extractTarEntries(tr *tar.Reader, destDir string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		target, err := safeTarTarget(destDir, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("unsupported link in tar archive: %s", hdr.Name)
		}
	}
	return nil
}
