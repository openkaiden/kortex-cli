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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// httpsToHTTPTransport rewrites every https:// request to http:// on the given host.
// Used to point OCI code (which hard-codes https://) at a plain httptest.Server.
type httpsToHTTPTransport struct{ host string }

func (t *httpsToHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme = "http"
	u.Host = t.host
	req2 := req.Clone(req.Context())
	req2.URL = &u
	return http.DefaultTransport.RoundTrip(req2)
}

// errTransport always returns the given error from RoundTrip.
type errTransport struct{ err error }

func (t *errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

// ociClientFor returns an *http.Client that redirects https:// calls to the httptest server.
func ociClientFor(srv *httptest.Server) *http.Client {
	host := strings.TrimPrefix(srv.URL, "http://")
	return &http.Client{Transport: &httpsToHTTPTransport{host: host}}
}

// --- parseOCIRef ---

func TestParseOCIRef(t *testing.T) {
	t.Parallel()

	cases := []struct {
		id         string
		registry   string
		repository string
		ref        string
	}{
		// Explicit registry with tag
		{"ghcr.io/org/feature:1", "ghcr.io", "org/feature", "1"},
		// Explicit registry, no tag → latest
		{"ghcr.io/org/feature", "ghcr.io", "org/feature", "latest"},
		// Digest reference
		{"ghcr.io/org/feature@sha256:abc123", "ghcr.io", "org/feature", "sha256:abc123"},
		// Registry with port
		{"localhost:5000/myfeature:tag", "localhost:5000", "myfeature", "tag"},
		// "localhost" without port is still treated as registry
		{"localhost/myfeature:tag", "localhost", "myfeature", "tag"},
		// No explicit registry → defaults to ghcr.io
		{"myorg/feature:1.0", "ghcr.io", "myorg/feature", "1.0"},
		// No registry, no tag
		{"myorg/feature", "ghcr.io", "myorg/feature", "latest"},
		// Single bare name (no slash, no tag)
		{"singlename", "ghcr.io", "singlename", "latest"},
		// Registry with dot, no tag
		{"registry.example.com/org/feature", "registry.example.com", "org/feature", "latest"},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			registry, repository, ref, err := parseOCIRef(tc.id)
			if err != nil {
				t.Fatalf("parseOCIRef(%q) error = %v", tc.id, err)
			}
			if registry != tc.registry {
				t.Errorf("registry = %q, want %q", registry, tc.registry)
			}
			if repository != tc.repository {
				t.Errorf("repository = %q, want %q", repository, tc.repository)
			}
			if ref != tc.ref {
				t.Errorf("ref = %q, want %q", ref, tc.ref)
			}
		})
	}
}

// --- parseWWWAuthenticate ---

func TestParseWWWAuthenticate(t *testing.T) {
	t.Parallel()

	t.Run("full header with realm service and scope", func(t *testing.T) {
		t.Parallel()
		realm, service, scope := parseWWWAuthenticate(
			`Bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:org/feature:pull"`,
			"org/feature",
		)
		if realm != "https://ghcr.io/token" {
			t.Errorf("realm = %q, want %q", realm, "https://ghcr.io/token")
		}
		if service != "ghcr.io" {
			t.Errorf("service = %q, want %q", service, "ghcr.io")
		}
		if scope != "repository:org/feature:pull" {
			t.Errorf("scope = %q, want %q", scope, "repository:org/feature:pull")
		}
	})

	t.Run("no scope in header uses constructed fallback", func(t *testing.T) {
		t.Parallel()
		_, _, scope := parseWWWAuthenticate(
			`Bearer realm="https://auth.example.com/token",service="example.com"`,
			"myorg/myrepo",
		)
		want := "repository:myorg/myrepo:pull"
		if scope != want {
			t.Errorf("scope = %q, want %q", scope, want)
		}
	})

	t.Run("no service field returns empty service", func(t *testing.T) {
		t.Parallel()
		_, service, _ := parseWWWAuthenticate(
			`Bearer realm="https://auth.example.com/token"`,
			"myrepo",
		)
		if service != "" {
			t.Errorf("service = %q, want empty", service)
		}
	})

	t.Run("empty header returns empty realm", func(t *testing.T) {
		t.Parallel()
		realm, _, _ := parseWWWAuthenticate("", "myrepo")
		if realm != "" {
			t.Errorf("realm = %q, want empty", realm)
		}
	})

	t.Run("header without Bearer prefix is still parsed", func(t *testing.T) {
		t.Parallel()
		realm, service, _ := parseWWWAuthenticate(
			`realm="https://token.example.com",service="example"`,
			"myrepo",
		)
		if realm != "https://token.example.com" {
			t.Errorf("realm = %q, want %q", realm, "https://token.example.com")
		}
		if service != "example" {
			t.Errorf("service = %q, want %q", service, "example")
		}
	})
}

// --- fetchBearerToken ---

func TestFetchBearerToken_SuccessTokenField(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"token":"mytoken123"}`)
	}))
	defer srv.Close()

	wwwAuth := fmt.Sprintf(`Bearer realm="%s/token",service="test"`, srv.URL)
	token, err := fetchBearerToken(context.Background(), http.DefaultClient, wwwAuth, "org/repo")
	if err != nil {
		t.Fatalf("fetchBearerToken: %v", err)
	}
	if token != "mytoken123" {
		t.Errorf("token = %q, want %q", token, "mytoken123")
	}
}

func TestFetchBearerToken_SuccessAccessTokenField(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// "access_token" field instead of "token"
		fmt.Fprint(w, `{"access_token":"accesstok456"}`)
	}))
	defer srv.Close()

	wwwAuth := fmt.Sprintf(`Bearer realm="%s/token",service="test"`, srv.URL)
	token, err := fetchBearerToken(context.Background(), http.DefaultClient, wwwAuth, "org/repo")
	if err != nil {
		t.Fatalf("fetchBearerToken: %v", err)
	}
	if token != "accesstok456" {
		t.Errorf("token = %q, want %q", token, "accesstok456")
	}
}

func TestFetchBearerToken_NoRealm(t *testing.T) {
	t.Parallel()

	_, err := fetchBearerToken(context.Background(), http.DefaultClient, "", "org/repo")
	if err == nil {
		t.Fatal("expected error for missing realm, got nil")
	}
	if !strings.Contains(err.Error(), "no realm") {
		t.Errorf("error = %q, want to contain 'no realm'", err.Error())
	}
}

func TestFetchBearerToken_TokenEndpointNonOK(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	wwwAuth := fmt.Sprintf(`Bearer realm="%s/token",service="test"`, srv.URL)
	_, err := fetchBearerToken(context.Background(), http.DefaultClient, wwwAuth, "org/repo")
	if err == nil {
		t.Fatal("expected error for non-200 token response, got nil")
	}
	if !strings.Contains(err.Error(), "token fetch failed") {
		t.Errorf("error = %q, want to contain 'token fetch failed'", err.Error())
	}
}

func TestFetchBearerToken_TokenEndpointBadJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not valid json")
	}))
	defer srv.Close()

	wwwAuth := fmt.Sprintf(`Bearer realm="%s/token",service="test"`, srv.URL)
	_, err := fetchBearerToken(context.Background(), http.DefaultClient, wwwAuth, "org/repo")
	if err == nil {
		t.Fatal("expected error for bad JSON, got nil")
	}
}

func TestFetchBearerToken_NetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	networkErr := errors.New("simulated network failure")
	wwwAuth := fmt.Sprintf(`Bearer realm="%s/token",service="test"`, srv.URL)
	_, err := fetchBearerToken(
		context.Background(),
		&http.Client{Transport: &errTransport{err: networkErr}},
		wwwAuth,
		"org/repo",
	)
	if !errors.Is(err, networkErr) {
		t.Errorf("error = %v, want %v", err, networkErr)
	}
}

// --- fetchManifest ---

func TestFetchManifest_NoAuth(t *testing.T) {
	t.Parallel()

	manifest := `{"layers":[{"digest":"sha256:aaa"},{"digest":"sha256:bbb"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		fmt.Fprint(w, manifest)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	m, token, err := feat.fetchManifest(context.Background(), host, "org/feat", "latest")
	if err != nil {
		t.Fatalf("fetchManifest: %v", err)
	}
	if token != "" {
		t.Errorf("token = %q, want empty (no auth required)", token)
	}
	if len(m.Layers) != 2 {
		t.Errorf("len(layers) = %d, want 2", len(m.Layers))
	}
}

func TestFetchManifest_WithBearerAuth(t *testing.T) {
	t.Parallel()

	manifest := `{"layers":[{"digest":"sha256:abc"}]}`
	var reqCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&reqCount, 1)
		switch {
		case strings.HasSuffix(r.URL.Path, "/manifests/latest"):
			if n == 1 {
				// First manifest request: issue Bearer challenge.
				w.Header().Set("WWW-Authenticate",
					fmt.Sprintf(`Bearer realm="http://%s/token",service="test"`, r.Host))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// Subsequent request: verify token and serve manifest.
			if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
				http.Error(w, "bad auth", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			fmt.Fprint(w, manifest)
		case r.URL.Path == "/token":
			fmt.Fprint(w, `{"token":"tok123"}`)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	m, token, err := feat.fetchManifest(context.Background(), host, "org/feat", "latest")
	if err != nil {
		t.Fatalf("fetchManifest: %v", err)
	}
	if token != "tok123" {
		t.Errorf("token = %q, want %q", token, "tok123")
	}
	if len(m.Layers) != 1 {
		t.Errorf("len(layers) = %d, want 1", len(m.Layers))
	}
}

func TestFetchManifest_NonOKStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	_, _, err := feat.fetchManifest(context.Background(), host, "org/feat", "latest")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !strings.Contains(err.Error(), "manifest fetch failed") {
		t.Errorf("error = %q, want to contain 'manifest fetch failed'", err.Error())
	}
}

func TestFetchManifest_BadJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		fmt.Fprint(w, "not json at all")
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	_, _, err := feat.fetchManifest(context.Background(), host, "org/feat", "latest")
	if err == nil {
		t.Fatal("expected error for bad JSON manifest, got nil")
	}
	if !strings.Contains(err.Error(), "decoding manifest") {
		t.Errorf("error = %q, want to contain 'decoding manifest'", err.Error())
	}
}

func TestFetchManifest_NetworkError(t *testing.T) {
	t.Parallel()

	networkErr := errors.New("dial failed")
	feat := &ociFeature{
		id:     "ghcr.io/org/feat:latest",
		client: &http.Client{Transport: &errTransport{err: networkErr}},
	}
	_, _, err := feat.fetchManifest(context.Background(), "ghcr.io", "org/feat", "latest")
	if !errors.Is(err, networkErr) {
		t.Errorf("error = %v, want %v", err, networkErr)
	}
}

func TestFetchManifest_AuthTokenFetchFails(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			// Always issue a challenge; token endpoint will fail.
			w.Header().Set("WWW-Authenticate",
				fmt.Sprintf(`Bearer realm="http://%s/token",service="test"`, r.Host))
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/token":
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	_, _, err := feat.fetchManifest(context.Background(), host, "org/feat", "latest")
	if err == nil {
		t.Fatal("expected error when token fetch fails, got nil")
	}
	if !strings.Contains(err.Error(), "fetching bearer token") {
		t.Errorf("error = %q, want to contain 'fetching bearer token'", err.Error())
	}
}

// --- downloadAndExtractLayer ---

func TestDownloadAndExtractLayer_NonOKBlob(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	err := feat.downloadAndExtractLayer(
		context.Background(), host, "org/feat", "sha256:missing", "", t.TempDir(),
	)
	if err == nil {
		t.Fatal("expected error for 404 blob, got nil")
	}
	if !strings.Contains(err.Error(), "blob fetch failed") {
		t.Errorf("error = %q, want to contain 'blob fetch failed'", err.Error())
	}
}

func TestDownloadAndExtractLayer_WithToken(t *testing.T) {
	t.Parallel()

	// Serve a minimal plain tar containing one file.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	content := []byte("hello")
	_ = tw.WriteHeader(&tar.Header{Name: "hello.txt", Mode: 0644, Size: int64(len(content))})
	_, _ = tw.Write(content)
	tw.Close()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write(tarBuf.Bytes())
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	destDir := t.TempDir()
	err := feat.downloadAndExtractLayer(
		context.Background(), host, "org/feat", "sha256:abc", "mytoken", destDir,
	)
	if err != nil {
		t.Fatalf("downloadAndExtractLayer: %v", err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer mytoken")
	}
	if _, err := os.Stat(filepath.Join(destDir, "hello.txt")); err != nil {
		t.Errorf("hello.txt not found: %v", err)
	}
}

// --- extractTar ---

func TestExtractTar_EmptyStream(t *testing.T) {
	t.Parallel()

	err := extractTar(bytes.NewReader(nil), t.TempDir())
	if err != nil {
		t.Errorf("expected nil error for empty stream, got %v", err)
	}
}

func TestExtractTar_PlainTar(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	content := []byte("plain tar content")
	_ = tw.WriteHeader(&tar.Header{Name: "plain.txt", Mode: 0644, Size: int64(len(content))})
	_, _ = tw.Write(content)
	tw.Close()

	destDir := t.TempDir()
	if err := extractTar(&buf, destDir); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "plain.txt"))
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestExtractTar_GzipTar(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	content := []byte("gzip tar content")
	_ = tw.WriteHeader(&tar.Header{Name: "gzip.txt", Mode: 0644, Size: int64(len(content))})
	_, _ = tw.Write(content)
	tw.Close()
	gw.Close()

	destDir := t.TempDir()
	if err := extractTar(&buf, destDir); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "gzip.txt"))
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestExtractTar_InvalidGzip(t *testing.T) {
	t.Parallel()

	// Gzip magic bytes followed by an invalid/truncated header.
	data := []byte{0x1f, 0x8b, 0x00}
	err := extractTar(bytes.NewReader(data), t.TempDir())
	if err == nil {
		t.Error("expected error for invalid gzip, got nil")
	}
	if !strings.Contains(err.Error(), "creating gzip reader") {
		t.Errorf("error = %q, want to contain 'creating gzip reader'", err.Error())
	}
}

// --- extractTarEntries ---

func TestExtractTarEntries_DirectoryEntry(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "mydir/", Mode: 0755})
	tw.Close()

	destDir := t.TempDir()
	if err := extractTarEntries(tar.NewReader(&buf), destDir); err != nil {
		t.Fatalf("extractTarEntries: %v", err)
	}
	fi, err := os.Stat(filepath.Join(destDir, "mydir"))
	if err != nil {
		t.Fatalf("mydir not found: %v", err)
	}
	if !fi.IsDir() {
		t.Error("mydir is not a directory")
	}
}

func TestExtractTarEntries_SymlinkEntryRejected(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "link.txt",
		Linkname: "target.txt",
	})
	tw.Close()

	destDir := t.TempDir()
	if err := extractTarEntries(tar.NewReader(&buf), destDir); err == nil {
		t.Fatal("expected error for symlink entry, got nil")
	}
}

func TestExtractTarEntries_PathTraversalRejected(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	content := []byte("evil")
	// Use a path that after filepath.Clean resolves to something starting with ".."
	_ = tw.WriteHeader(&tar.Header{Name: "../evil.txt", Mode: 0644, Size: int64(len(content))})
	_, _ = tw.Write(content)
	tw.Close()

	err := extractTarEntries(tar.NewReader(&buf), t.TempDir())
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "invalid path") {
		t.Errorf("error = %q, want to contain 'invalid path'", err.Error())
	}
}

func TestExtractTarEntries_TarReadError(t *testing.T) {
	t.Parallel()

	// Feed corrupted data that causes tar.Reader.Next() to return a non-EOF error.
	garbage := bytes.Repeat([]byte{0xFF}, 600) // enough to pass the 512-byte header read but invalid
	err := extractTarEntries(tar.NewReader(bytes.NewReader(garbage)), t.TempDir())
	if err == nil {
		t.Fatal("expected error from corrupt tar data, got nil")
	}
	if !strings.Contains(err.Error(), "reading tar entry") {
		t.Errorf("error = %q, want to contain 'reading tar entry'", err.Error())
	}
}

// --- ociFeature.Download error paths ---

func TestOCIFeatureDownload_ManifestFetchError(t *testing.T) {
	t.Parallel()

	networkErr := errors.New("connection refused")
	feat := &ociFeature{
		id:     "ghcr.io/org/feat:latest",
		client: &http.Client{Transport: &errTransport{err: networkErr}},
	}
	_, err := feat.Download(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected error when manifest fetch fails, got nil")
	}
	if !strings.Contains(err.Error(), "fetching manifest") {
		t.Errorf("error = %q, want to contain 'fetching manifest'", err.Error())
	}
}

func TestOCIFeatureDownload_LayerExtractError(t *testing.T) {
	t.Parallel()

	manifest := `{"layers":[{"digest":"sha256:bad"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			fmt.Fprint(w, manifest)
		case strings.Contains(r.URL.Path, "/blobs/"):
			http.Error(w, "blob gone", http.StatusGone)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	_, err := feat.Download(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected error when layer extraction fails, got nil")
	}
	if !strings.Contains(err.Error(), "extracting layer") {
		t.Errorf("error = %q, want to contain 'extracting layer'", err.Error())
	}
}

func TestOCIFeatureDownload_MissingFeatureJSON(t *testing.T) {
	t.Parallel()

	// Serve a manifest with one layer that extracts a tar containing no devcontainer-feature.json.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	body := []byte("just a script")
	_ = tw.WriteHeader(&tar.Header{Name: "install.sh", Mode: 0755, Size: int64(len(body))})
	_, _ = tw.Write(body)
	tw.Close()
	tarBytes := tarBuf.Bytes()

	manifest, _ := json.Marshal(map[string]interface{}{
		"layers": []map[string]interface{}{{"digest": "sha256:only"}},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Write(manifest)
		case strings.Contains(r.URL.Path, "/blobs/"):
			w.Write(tarBytes)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	feat := &ociFeature{id: host + "/org/feat:latest", client: ociClientFor(srv)}

	_, err := feat.Download(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected error when devcontainer-feature.json is absent, got nil")
	}
	if !strings.Contains(err.Error(), "devcontainer-feature.json") {
		t.Errorf("error = %q, want to contain 'devcontainer-feature.json'", err.Error())
	}
}
