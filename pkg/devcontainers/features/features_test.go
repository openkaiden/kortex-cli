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

package features_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkaiden/kdn/pkg/devcontainers/features"
)

// --- helpers ---

func writeFeatureJSON(t *testing.T, dir string, content map[string]interface{}) {
	t.Helper()
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "devcontainer-feature.json"), data, 0644); err != nil {
		t.Fatalf("write devcontainer-feature.json: %v", err)
	}
}

// makeLocalFeatureDir creates workspaceDir/<relPath>/devcontainer-feature.json with the given spec.
func makeLocalFeatureDir(t *testing.T, workspaceDir, relPath string, spec map[string]interface{}) {
	t.Helper()
	featureDir := filepath.Join(workspaceDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFeatureJSON(t, featureDir, spec)
}

// metadataFromLocalFeature downloads a local feature and returns its metadata.
func metadataFromLocalFeature(t *testing.T, workspaceDir, featureID string) features.FeatureMetadata {
	t.Helper()
	feats, _, err := features.FromMap(
		map[string]map[string]interface{}{featureID: nil},
		workspaceDir,
	)
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	destDir := t.TempDir()
	meta, err := feats[0].Download(context.Background(), destDir)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	return meta
}

// --- fakes for Order tests ---

type fakeFeature struct {
	id string
}

func (f *fakeFeature) ID() string { return f.id }
func (f *fakeFeature) Download(_ context.Context, _ string) (features.FeatureMetadata, error) {
	return nil, nil
}

type fakeMetadata struct {
	installsAfter []string
}

func (m *fakeMetadata) ContainerEnv() map[string]string  { return nil }
func (m *fakeMetadata) Options() features.FeatureOptions { return nil }
func (m *fakeMetadata) InstallsAfter() []string          { return m.installsAfter }

// --- FeatureMetadata tests ---

func TestFeatureMetadata_InstallsAfter(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"installsAfter": []string{"ghcr.io/devcontainers/features/common-utils"},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	got := meta.InstallsAfter()
	if len(got) != 1 || got[0] != "ghcr.io/devcontainers/features/common-utils" {
		t.Errorf("InstallsAfter() = %v, want [ghcr.io/devcontainers/features/common-utils]", got)
	}
}

func TestFeatureMetadata_InstallsAfter_Empty(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	got := meta.InstallsAfter()
	if len(got) != 0 {
		t.Errorf("InstallsAfter() = %v, want []", got)
	}
}

// --- FeatureOptions.Merge tests ---

func TestFeatureOptions_Merge_DefaultsUsedWhenNoUserOpts(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"version": map[string]interface{}{
				"type":    "string",
				"default": "latest",
			},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	result, err := meta.Options().Merge(nil)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := result["VERSION"]; got != "latest" {
		t.Errorf("VERSION = %q, want %q", got, "latest")
	}
}

func TestFeatureOptions_Merge_UserOverridesDefault(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"version": map[string]interface{}{
				"type":    "string",
				"default": "latest",
			},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	result, err := meta.Options().Merge(map[string]interface{}{
		"version": "1.21",
	})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := result["VERSION"]; got != "1.21" {
		t.Errorf("VERSION = %q, want %q", got, "1.21")
	}
}

func TestFeatureOptions_Merge_BooleanDefaultCoercedToString(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"install-tools": map[string]interface{}{
				"type":    "boolean",
				"default": true,
			},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	result, err := meta.Options().Merge(nil)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := result["INSTALL_TOOLS"]; got != "true" {
		t.Errorf("INSTALL_TOOLS = %q, want %q", got, "true")
	}
}

func TestFeatureOptions_Merge_KeyNormalization(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"install-tools": map[string]interface{}{
				"type":    "boolean",
				"default": false,
			},
			"go.version": map[string]interface{}{
				"type":    "string",
				"default": "1.21",
			},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	result, err := meta.Options().Merge(nil)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if _, ok := result["INSTALL_TOOLS"]; !ok {
		t.Error("expected key INSTALL_TOOLS (from install-tools)")
	}
	if _, ok := result["GO_VERSION"]; !ok {
		t.Error("expected key GO_VERSION (from go.version)")
	}
}

func TestFeatureOptions_Merge_InvalidEnumReturnsError(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"flavor": map[string]interface{}{
				"type":    "string",
				"default": "basic",
				"enum":    []string{"none", "basic", "full"},
			},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	_, err := meta.Options().Merge(map[string]interface{}{
		"flavor": "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid enum value, got nil")
	}
}

func TestFeatureOptions_Merge_WrongTypeReturnsError(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"version": map[string]interface{}{
				"type":    "string",
				"default": "latest",
			},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	_, err := meta.Options().Merge(map[string]interface{}{
		"version": 42, // should be string
	})
	if err == nil {
		t.Error("expected error for wrong type, got nil")
	}
}

func TestFeatureOptions_Merge_UnknownOptionReturnsError(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"version": map[string]interface{}{"type": "string", "default": "latest"},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	_, err := meta.Options().Merge(map[string]interface{}{
		"unknown-key": "value",
	})
	if err == nil {
		t.Fatal("expected error for unknown option, got nil")
	}
	if !strings.Contains(err.Error(), "unknown option") {
		t.Errorf("error = %q, want to contain 'unknown option'", err.Error())
	}
}

func TestFeatureOptions_Merge_BooleanUserOptionTrue(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"install-tools": map[string]interface{}{"type": "boolean", "default": false},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	result, err := meta.Options().Merge(map[string]interface{}{
		"install-tools": true,
	})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := result["INSTALL_TOOLS"]; got != "true" {
		t.Errorf("INSTALL_TOOLS = %q, want %q", got, "true")
	}
}

func TestFeatureOptions_Merge_BooleanUserOptionFalse(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"install-tools": map[string]interface{}{"type": "boolean", "default": true},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	result, err := meta.Options().Merge(map[string]interface{}{
		"install-tools": false,
	})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := result["INSTALL_TOOLS"]; got != "false" {
		t.Errorf("INSTALL_TOOLS = %q, want %q", got, "false")
	}
}

func TestFeatureOptions_Merge_BooleanStringValueAccepted(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"install-tools": map[string]interface{}{"type": "boolean", "default": false},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	result, err := meta.Options().Merge(map[string]interface{}{
		"install-tools": "true",
	})
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := result["INSTALL_TOOLS"]; got != "true" {
		t.Errorf("INSTALL_TOOLS = %q, want %q", got, "true")
	}
}

func TestFeatureOptions_Merge_BooleanInvalidStringReturnsError(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"install-tools": map[string]interface{}{"type": "boolean", "default": false},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	_, err := meta.Options().Merge(map[string]interface{}{
		"install-tools": "yes",
	})
	if err == nil {
		t.Error("expected error for invalid boolean string, got nil")
	}
}

func TestFeatureOptions_Merge_BooleanWrongTypeReturnsError(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"install-tools": map[string]interface{}{"type": "boolean", "default": false},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	_, err := meta.Options().Merge(map[string]interface{}{
		"install-tools": 42,
	})
	if err == nil {
		t.Error("expected error for wrong type for boolean option, got nil")
	}
}

func TestFeatureOptions_Merge_UnsupportedTypeReturnsError(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{
		"options": map[string]interface{}{
			"count": map[string]interface{}{"type": "integer", "default": 1},
		},
	})

	meta := metadataFromLocalFeature(t, workspaceDir, "./my-feature")
	_, err := meta.Options().Merge(map[string]interface{}{
		"count": "3",
	})
	if err == nil {
		t.Fatal("expected error for unsupported option type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("error = %q, want to contain 'unsupported type'", err.Error())
	}
}

// --- FromMap tests ---

func TestFromMap_EmptyMap(t *testing.T) {
	t.Parallel()

	feats, opts, err := features.FromMap(map[string]map[string]interface{}{}, t.TempDir())
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if len(feats) != 0 {
		t.Errorf("len(feats) = %d, want 0", len(feats))
	}
	if len(opts) != 0 {
		t.Errorf("len(opts) = %d, want 0", len(opts))
	}
}

func TestFromMap_OCIFeaturesSortedByID(t *testing.T) {
	t.Parallel()

	m := map[string]map[string]interface{}{
		"ghcr.io/devcontainers/features/go:1":               nil,
		"ghcr.io/devcontainers/features/common-utils":       nil,
		"ghcr.io/devcontainers/features/docker-in-docker:2": nil,
	}

	feats, _, err := features.FromMap(m, t.TempDir())
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if len(feats) != 3 {
		t.Fatalf("len(feats) = %d, want 3", len(feats))
	}

	ids := idsOf(feats)
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Errorf("features not sorted by ID: %v", ids)
			break
		}
	}
}

func TestFromMap_LocalRelativePath(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	makeLocalFeatureDir(t, workspaceDir, "my-feature", map[string]interface{}{})

	feats, _, err := features.FromMap(
		map[string]map[string]interface{}{"./my-feature": nil},
		workspaceDir,
	)
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	if len(feats) != 1 {
		t.Fatalf("len(feats) = %d, want 1", len(feats))
	}
	if feats[0].ID() != "./my-feature" {
		t.Errorf("ID = %q, want %q", feats[0].ID(), "./my-feature")
	}

	// Verify Download works for a local feature.
	_, err = feats[0].Download(context.Background(), t.TempDir())
	if err != nil {
		t.Errorf("Download: %v", err)
	}
}

func TestLocalFeature_Download_InvalidJSON(t *testing.T) {
	t.Parallel()

	workspaceDir := t.TempDir()
	featureDir := filepath.Join(workspaceDir, "my-feature")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "devcontainer-feature.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	feats, _, err := features.FromMap(
		map[string]map[string]interface{}{"./my-feature": nil},
		workspaceDir,
	)
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}

	_, err = feats[0].Download(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error = %q, want to contain 'parsing'", err.Error())
	}
}

func TestFromMap_TgzURIReturnsError(t *testing.T) {
	t.Parallel()

	_, _, err := features.FromMap(
		map[string]map[string]interface{}{
			"https://example.com/feature.tgz": nil,
		},
		t.TempDir(),
	)
	if err == nil {
		t.Fatal("expected error for Tgz URI, got nil")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %q, want to contain 'not supported'", err.Error())
	}
}

func TestFromMap_UserOptionsPreserved(t *testing.T) {
	t.Parallel()

	m := map[string]map[string]interface{}{
		"ghcr.io/devcontainers/features/go:1": {"version": "1.21"},
	}

	_, opts, err := features.FromMap(m, t.TempDir())
	if err != nil {
		t.Fatalf("FromMap: %v", err)
	}
	v, ok := opts["ghcr.io/devcontainers/features/go:1"]
	if !ok {
		t.Fatal("user options missing for feature")
	}
	if v["version"] != "1.21" {
		t.Errorf("version = %v, want %q", v["version"], "1.21")
	}
}

// --- Order tests ---

func TestOrder_SingleFeatureNoInstallsAfter(t *testing.T) {
	t.Parallel()

	feats := []features.Feature{&fakeFeature{id: "A"}}
	metadata := map[string]features.FeatureMetadata{
		"A": &fakeMetadata{},
	}

	ordered, err := features.Order(feats, metadata)
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if len(ordered) != 1 || ordered[0].ID() != "A" {
		t.Errorf("ordered = %v, want [A]", idsOf(ordered))
	}
}

func TestOrder_MultipleWithInstallsAfterChain(t *testing.T) {
	t.Parallel()

	// C.installsAfter = [B], B.installsAfter = [A]  →  correct order: A, B, C
	feats := []features.Feature{
		&fakeFeature{id: "C"},
		&fakeFeature{id: "A"},
		&fakeFeature{id: "B"},
	}
	metadata := map[string]features.FeatureMetadata{
		"A": &fakeMetadata{},
		"B": &fakeMetadata{installsAfter: []string{"A"}},
		"C": &fakeMetadata{installsAfter: []string{"B"}},
	}

	ordered, err := features.Order(feats, metadata)
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	got := idsOf(ordered)
	want := []string{"A", "B", "C"}
	if !equalSlices(got, want) {
		t.Errorf("ordered = %v, want %v", got, want)
	}
}

func TestOrder_InstallsAfterExternalFeatureIgnored(t *testing.T) {
	t.Parallel()

	// Per the Dev Container spec, installsAfter is a soft ordering hint.
	// References to features not present in feats (e.g. features from a
	// different layer or not selected by the user) must be silently ignored,
	// not treated as errors. This is distinct from a missing metadata entry
	// (which is always an error because it means a feature in feats was not
	// downloaded before Order was called).
	feats := []features.Feature{&fakeFeature{id: "A"}}
	metadata := map[string]features.FeatureMetadata{
		"A": &fakeMetadata{installsAfter: []string{"external-feature"}},
	}

	ordered, err := features.Order(feats, metadata)
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if len(ordered) != 1 || ordered[0].ID() != "A" {
		t.Errorf("ordered = %v, want [A]", idsOf(ordered))
	}
}

func TestOrder_CycleDetectedReturnsError(t *testing.T) {
	t.Parallel()

	// A.installsAfter = [B] and B.installsAfter = [A] → cycle
	feats := []features.Feature{
		&fakeFeature{id: "A"},
		&fakeFeature{id: "B"},
	}
	metadata := map[string]features.FeatureMetadata{
		"A": &fakeMetadata{installsAfter: []string{"B"}},
		"B": &fakeMetadata{installsAfter: []string{"A"}},
	}

	_, err := features.Order(feats, metadata)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want to contain 'cycle'", err.Error())
	}
}

func TestOrder_MissingMetadataReturnsError(t *testing.T) {
	t.Parallel()

	feats := []features.Feature{&fakeFeature{id: "A"}}
	metadata := map[string]features.FeatureMetadata{} // A missing

	_, err := features.Order(feats, metadata)
	if err == nil {
		t.Error("expected error for missing metadata, got nil")
	}
}

// --- OCI download test using httptest ---

func TestOCIFeature_Download(t *testing.T) {
	t.Parallel()

	featureJSON := map[string]interface{}{
		"id":            "go",
		"containerEnv":  map[string]string{"GOPATH": "/go"},
		"installsAfter": []string{},
	}
	featureJSONBytes, _ := json.Marshal(featureJSON)

	// Plain tar layer containing devcontainer-feature.json and install.sh.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	addTarFile(t, tw, "devcontainer-feature.json", featureJSONBytes)
	addTarFile(t, tw, "install.sh", []byte("#!/bin/sh\necho hello"))
	if err := tw.Close(); err != nil {
		t.Fatalf("closing plain tar writer: %v", err)
	}
	plainTarBytes := tarBuf.Bytes()

	// Gzip tar layer containing extra.txt.
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	tw2 := tar.NewWriter(gw)
	addTarFile(t, tw2, "extra.txt", []byte("extra"))
	if err := tw2.Close(); err != nil {
		t.Fatalf("closing gzip tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}
	gzipTarBytes := gzBuf.Bytes()

	h1 := sha256.Sum256(plainTarBytes)
	digest1 := fmt.Sprintf("sha256:%x", h1)
	h2 := sha256.Sum256(gzipTarBytes)
	digest2 := fmt.Sprintf("sha256:%x", h2)

	manifest := map[string]interface{}{
		"schemaVersion": 2,
		"layers": []map[string]interface{}{
			{"digest": digest1, "mediaType": "application/vnd.devcontainers.layer.v1+tar"},
			{"digest": digest2, "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip"},
		},
	}
	manifestBytes, _ := json.Marshal(manifest)

	blobs := map[string][]byte{
		digest1: plainTarBytes,
		digest2: gzipTarBytes,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			_, _ = w.Write(manifestBytes)
		case strings.Contains(r.URL.Path, "/blobs/"):
			parts := strings.SplitAfter(r.URL.Path, "/blobs/")
			digest := parts[len(parts)-1]
			data, ok := blobs[digest]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// The test server host (e.g. "127.0.0.1:PORT") contains ":" → treated as registry.
	host := strings.TrimPrefix(srv.URL, "http://")
	featureID := host + "/test/feature:latest"

	// Inject a transport that rewrites https:// to http:// for the test server.
	feat := features.NewOCIFeatureWithClient(featureID, &http.Client{
		Transport: &rewriteTransport{host: host},
	})

	destDir := t.TempDir()
	meta, err := feat.Download(context.Background(), destDir)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	if got := meta.ContainerEnv()["GOPATH"]; got != "/go" {
		t.Errorf("ContainerEnv GOPATH = %q, want %q", got, "/go")
	}

	if _, err := os.Stat(filepath.Join(destDir, "install.sh")); err != nil {
		t.Errorf("install.sh not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "extra.txt")); err != nil {
		t.Errorf("extra.txt not found: %v", err)
	}
}

// rewriteTransport rewrites https:// requests to http:// on the same host.
type rewriteTransport struct {
	host string
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme = "http"
	u.Host = rt.host
	req2 := req.Clone(req.Context())
	req2.URL = &u
	return http.DefaultTransport.RoundTrip(req2)
}

// --- test helpers ---

func addTarFile(t *testing.T, tw *tar.Writer, name string, content []byte) {
	t.Helper()
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar WriteHeader: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar Write: %v", err)
	}
}

func idsOf(feats []features.Feature) []string {
	ids := make([]string, len(feats))
	for i, f := range feats {
		ids[i] = f.ID()
	}
	return ids
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
