// Copyright 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openshell

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	workspace "github.com/openkaiden/kdn-api/workspace-configuration/go"
	"github.com/openkaiden/kdn/pkg/credential"
	"github.com/openkaiden/kdn/pkg/credential/gcloud"
	"github.com/openkaiden/kdn/pkg/onecli"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/runtime/openshell/exec"
)

func TestInterceptCredentials_NilRegistry(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())

	mounts := []workspace.Mount{{Host: "$HOME/.config/gcloud", Target: "$HOME/.config/gcloud"}}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn != nil {
		t.Error("expected nil uploadFn")
	}
}

func TestInterceptCredentials_NilConfig(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.credentialRegistry = credential.NewRegistry()

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn != nil {
		t.Error("expected nil uploadFn")
	}
}

func TestInterceptCredentials_NoMounts(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	rt.credentialRegistry = credential.NewRegistry()

	cfg := &workspace.WorkspaceConfiguration{}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn != nil {
		t.Error("expected nil uploadFn")
	}
}

func TestInterceptCredentials_NoGcloudMount(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(gcloud.New())
	rt.credentialRegistry = reg

	mounts := []workspace.Mount{{Host: "$HOME/projects", Target: "$HOME/projects"}}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn != nil {
		t.Error("expected nil uploadFn")
	}
}

func TestInterceptCredentials_DetectsGcloudMount(t *testing.T) {
	t.Parallel()

	adcDir := filepath.Join(t.TempDir(), ".config", "gcloud")
	if err := os.MkdirAll(adcDir, 0755); err != nil {
		t.Fatalf("failed to create ADC dir: %v", err)
	}
	adcPath := filepath.Join(adcDir, "application_default_credentials.json")
	adcContent := `{
		"client_id": "test-client-id",
		"client_secret": "test-client-secret",
		"refresh_token": "test-refresh-token",
		"type": "authorized_user"
	}`
	if err := os.WriteFile(adcPath, []byte(adcContent), 0644); err != nil {
		t.Fatalf("failed to write ADC file: %v", err)
	}

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(gcloud.New())
	rt.credentialRegistry = reg

	mounts := []workspace.Mount{
		{Host: adcPath, Target: "$HOME/.config/gcloud/application_default_credentials.json"},
	}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn == nil {
		t.Fatal("expected non-nil uploadFn")
	}
}

func TestInterceptCredentials_RemovesInterceptedMount(t *testing.T) {
	t.Parallel()

	adcDir := filepath.Join(t.TempDir(), ".config", "gcloud")
	if err := os.MkdirAll(adcDir, 0755); err != nil {
		t.Fatalf("failed to create ADC dir: %v", err)
	}
	adcPath := filepath.Join(adcDir, "application_default_credentials.json")
	if err := os.WriteFile(adcPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write ADC file: %v", err)
	}

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(gcloud.New())
	rt.credentialRegistry = reg

	mounts := []workspace.Mount{
		{Host: "$HOME/projects", Target: "$HOME/projects"},
		{Host: adcPath, Target: "$HOME/.config/gcloud/application_default_credentials.json"},
		{Host: "$HOME/other", Target: "$HOME/other"},
	}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	_, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*cfg.Mounts) != 2 {
		t.Fatalf("expected 2 mounts after interception, got %d", len(*cfg.Mounts))
	}
	for _, m := range *cfg.Mounts {
		if strings.Contains(m.Target, "gcloud") {
			t.Errorf("gcloud mount should have been removed, found: %+v", m)
		}
	}
}

func TestInterceptCredentials_AddsVertexHosts(t *testing.T) {
	t.Parallel()

	adcDir := filepath.Join(t.TempDir(), ".config", "gcloud")
	if err := os.MkdirAll(adcDir, 0755); err != nil {
		t.Fatalf("failed to create ADC dir: %v", err)
	}
	adcPath := filepath.Join(adcDir, "application_default_credentials.json")
	if err := os.WriteFile(adcPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write ADC file: %v", err)
	}

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(gcloud.New())
	rt.credentialRegistry = reg

	existingHosts := []string{"registry.npmjs.org"}
	deny := workspace.Deny
	mounts := []workspace.Mount{
		{Host: adcPath, Target: "$HOME/.config/gcloud/application_default_credentials.json"},
	}
	cfg := &workspace.WorkspaceConfiguration{
		Mounts:  &mounts,
		Network: &workspace.NetworkConfiguration{Mode: &deny, Hosts: &existingHosts},
	}

	_, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Network == nil || cfg.Network.Hosts == nil {
		t.Fatal("expected network hosts to be set")
	}

	hosts := *cfg.Network.Hosts
	for _, want := range []string{"oauth2.googleapis.com", "aiplatform.googleapis.com", "us-central1-aiplatform.googleapis.com", "europe-west4-aiplatform.googleapis.com", "storage.googleapis.com"} {
		if !slices.Contains(hosts, want) {
			t.Errorf("missing host %q in %v", want, hosts)
		}
	}

	for _, h := range hosts {
		if h == "*-aiplatform.googleapis.com" {
			t.Error("wildcard *-aiplatform.googleapis.com should have been expanded")
		}
	}

	if !slices.Contains(hosts, "registry.npmjs.org") {
		t.Error("original host should still be present")
	}
}

func TestInterceptCredentials_HostFileNotFound(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(gcloud.New())
	rt.credentialRegistry = reg

	mounts := []workspace.Mount{
		{Host: "/nonexistent/path/application_default_credentials.json", Target: "$HOME/.config/gcloud/application_default_credentials.json"},
	}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn != nil {
		t.Error("expected nil uploadFn for missing file")
	}
}

func TestInterceptCredentials_UploadFnCallsSandboxUpload(t *testing.T) {
	t.Parallel()

	adcDir := filepath.Join(t.TempDir(), ".config", "gcloud")
	if err := os.MkdirAll(adcDir, 0755); err != nil {
		t.Fatalf("failed to create ADC dir: %v", err)
	}
	adcPath := filepath.Join(adcDir, "application_default_credentials.json")
	if err := os.WriteFile(adcPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write ADC file: %v", err)
	}

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(gcloud.New())
	rt.credentialRegistry = reg

	mounts := []workspace.Mount{
		{Host: adcPath, Target: "$HOME/.config/gcloud/application_default_credentials.json"},
	}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn == nil {
		t.Fatal("expected non-nil uploadFn")
	}

	if err := uploadFn(context.Background(), "kdn-test-sandbox"); err != nil {
		t.Fatalf("uploadFn failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 1 {
		t.Fatalf("expected 1 Run call for upload, got %d", len(fakeExec.RunCalls))
	}

	uploadCall := fakeExec.RunCalls[0]
	assertContainsAll(t, uploadCall, "sandbox", "upload", "kdn-test-sandbox", adcPath)

	lastArg := uploadCall[len(uploadCall)-1]
	if lastArg != "/sandbox/.config/gcloud/application_default_credentials.json" {
		t.Errorf("upload destination = %q, want %q", lastArg, "/sandbox/.config/gcloud/application_default_credentials.json")
	}
}

func TestInterceptCredentials_AddsHostsToNilNetwork(t *testing.T) {
	t.Parallel()

	adcDir := filepath.Join(t.TempDir(), ".config", "gcloud")
	if err := os.MkdirAll(adcDir, 0755); err != nil {
		t.Fatalf("failed to create ADC dir: %v", err)
	}
	adcPath := filepath.Join(adcDir, "application_default_credentials.json")
	if err := os.WriteFile(adcPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write ADC file: %v", err)
	}

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(gcloud.New())
	rt.credentialRegistry = reg

	mounts := []workspace.Mount{
		{Host: adcPath, Target: "$HOME/.config/gcloud/application_default_credentials.json"},
	}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	_, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Network == nil || cfg.Network.Hosts == nil {
		t.Fatal("expected network hosts to be initialized")
	}

	hosts := *cfg.Network.Hosts
	if len(hosts) == 0 {
		t.Error("expected Vertex AI hosts to be added")
	}
}

func TestRemoveMountFromConfig(t *testing.T) {
	t.Parallel()

	mounts := []workspace.Mount{
		{Host: "/a", Target: "/a"},
		{Host: "/b", Target: "/b"},
		{Host: "/c", Target: "/c"},
	}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	removeMountFromConfig(cfg, &(*cfg.Mounts)[1])

	if len(*cfg.Mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(*cfg.Mounts))
	}
	for _, m := range *cfg.Mounts {
		if m.Host == "/b" {
			t.Error("mount /b should have been removed")
		}
	}
}

func TestAddHostsToNetworkConfig(t *testing.T) {
	t.Parallel()

	t.Run("creates network config if nil", func(t *testing.T) {
		t.Parallel()

		cfg := &workspace.WorkspaceConfiguration{}
		addHostsToNetworkConfig(cfg, []string{"example.com"})

		if cfg.Network == nil || cfg.Network.Hosts == nil {
			t.Fatal("expected network hosts to be created")
		}
		if len(*cfg.Network.Hosts) != 1 || (*cfg.Network.Hosts)[0] != "example.com" {
			t.Errorf("hosts = %v, want [example.com]", *cfg.Network.Hosts)
		}
	})

	t.Run("deduplicates hosts", func(t *testing.T) {
		t.Parallel()

		hosts := []string{"existing.com"}
		cfg := &workspace.WorkspaceConfiguration{
			Network: &workspace.NetworkConfiguration{Hosts: &hosts},
		}

		addHostsToNetworkConfig(cfg, []string{"existing.com", "new.com"})

		if len(*cfg.Network.Hosts) != 2 {
			t.Errorf("expected 2 hosts, got %d: %v", len(*cfg.Network.Hosts), *cfg.Network.Hosts)
		}
	})

	t.Run("no-op for empty hosts", func(t *testing.T) {
		t.Parallel()

		cfg := &workspace.WorkspaceConfiguration{}
		addHostsToNetworkConfig(cfg, nil)

		if cfg.Network != nil {
			t.Error("expected network to remain nil")
		}
	})
}

func TestRemapContainerPath(t *testing.T) {
	t.Parallel()

	t.Run("remaps /home/agent prefix", func(t *testing.T) {
		t.Parallel()

		got := remapContainerPath("/home/agent/.config/gcloud/application_default_credentials.json")
		want := "/sandbox/.config/gcloud/application_default_credentials.json"
		if got != want {
			t.Errorf("remapContainerPath() = %q, want %q", got, want)
		}
	})

	t.Run("remaps /home/agent/.kube/config", func(t *testing.T) {
		t.Parallel()

		got := remapContainerPath("/home/agent/.kube/config")
		want := "/sandbox/.kube/config"
		if got != want {
			t.Errorf("remapContainerPath() = %q, want %q", got, want)
		}
	})

	t.Run("preserves paths without /home/agent prefix", func(t *testing.T) {
		t.Parallel()

		got := remapContainerPath("/etc/ssl/certs/ca-certificates.crt")
		want := "/etc/ssl/certs/ca-certificates.crt"
		if got != want {
			t.Errorf("remapContainerPath() = %q, want %q", got, want)
		}
	})
}

// fakeCredential implements credential.Credential for testing.
type fakeCredential struct {
	name              string
	containerFilePath string
	hostPatterns      []string
	detectHostPath    string
	detectTarget      string
}

var _ credential.Credential = (*fakeCredential)(nil)

func (f *fakeCredential) Name() string                   { return f.name }
func (f *fakeCredential) ContainerFilePath() string      { return f.containerFilePath }
func (f *fakeCredential) HostPatterns(_ string) []string { return f.hostPatterns }
func (f *fakeCredential) FakeFile(_ string) ([]byte, error) {
	return nil, nil
}
func (f *fakeCredential) Detect(mounts []workspace.Mount, _ string) (string, *workspace.Mount) {
	if f.detectTarget == "" {
		return "", nil
	}
	for i := range mounts {
		if mounts[i].Target == f.detectTarget {
			return f.detectHostPath, &mounts[i]
		}
	}
	return "", nil
}
func (f *fakeCredential) Configure(_ context.Context, _ onecli.Client, _ string) error { return nil }

func TestInterceptCredentials_UndetectedCredentialSkipped(t *testing.T) {
	t.Parallel()

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(&fakeCredential{
		name:              "custom",
		containerFilePath: "/home/agent/.custom/creds",
	})
	rt.credentialRegistry = reg

	mounts := []workspace.Mount{{Host: "/some/path", Target: "/some/target"}}
	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn != nil {
		t.Error("expected nil uploadFn when credential is not detected")
	}
}

func TestInterceptCredentials_DetectedCredentialUploaded(t *testing.T) {
	t.Parallel()

	credFile := filepath.Join(t.TempDir(), "my-creds.json")
	if err := os.WriteFile(credFile, []byte(`{"token":"test"}`), 0644); err != nil {
		t.Fatalf("failed to write credential file: %v", err)
	}

	mounts := []workspace.Mount{{Host: credFile, Target: "$HOME/.custom/creds"}}

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(&fakeCredential{
		name:              "custom",
		containerFilePath: "/home/agent/.custom/creds",
		hostPatterns:      []string{"api.custom.com"},
		detectHostPath:    credFile,
		detectTarget:      "$HOME/.custom/creds",
	})
	rt.credentialRegistry = reg

	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn == nil {
		t.Fatal("expected non-nil uploadFn for detected credential")
	}

	if err := uploadFn(context.Background(), "kdn-test"); err != nil {
		t.Fatalf("uploadFn failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 1 {
		t.Fatalf("expected 1 upload call, got %d", len(fakeExec.RunCalls))
	}

	call := fakeExec.RunCalls[0]
	lastArg := call[len(call)-1]
	if lastArg != "/sandbox/.custom/creds" {
		t.Errorf("upload destination = %q, want %q", lastArg, "/sandbox/.custom/creds")
	}
}

func TestInterceptCredentials_MultipleCredentials(t *testing.T) {
	t.Parallel()

	credFile1 := filepath.Join(t.TempDir(), "cred1.json")
	credFile2 := filepath.Join(t.TempDir(), "cred2.json")
	if err := os.WriteFile(credFile1, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write cred1: %v", err)
	}
	if err := os.WriteFile(credFile2, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write cred2: %v", err)
	}

	mounts := []workspace.Mount{
		{Host: credFile1, Target: "$HOME/.cred1"},
		{Host: credFile2, Target: "$HOME/.cred2"},
	}

	fakeExec := exec.NewFake()
	rt := newWithDeps(fakeExec, "/fake/gw", t.TempDir())
	reg := credential.NewRegistry()
	_ = reg.Register(&fakeCredential{
		name:              "cred-a",
		containerFilePath: "/home/agent/.cred1",
		hostPatterns:      []string{"api.a.com"},
		detectHostPath:    credFile1,
		detectTarget:      "$HOME/.cred1",
	})
	_ = reg.Register(&fakeCredential{
		name:              "cred-b",
		containerFilePath: "/home/agent/.cred2",
		hostPatterns:      []string{"api.b.com"},
		detectHostPath:    credFile2,
		detectTarget:      "$HOME/.cred2",
	})
	rt.credentialRegistry = reg

	cfg := &workspace.WorkspaceConfiguration{Mounts: &mounts}

	uploadFn, err := rt.interceptCredentials(context.Background(), runtime.CreateParams{WorkspaceConfig: cfg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadFn == nil {
		t.Fatal("expected non-nil uploadFn")
	}

	if err := uploadFn(context.Background(), "kdn-test"); err != nil {
		t.Fatalf("uploadFn failed: %v", err)
	}

	if len(fakeExec.RunCalls) != 2 {
		t.Fatalf("expected 2 upload calls, got %d", len(fakeExec.RunCalls))
	}

	if len(*cfg.Mounts) != 0 {
		t.Errorf("expected all mounts removed, got %d", len(*cfg.Mounts))
	}

	if cfg.Network == nil || cfg.Network.Hosts == nil {
		t.Fatal("expected network hosts to be set")
	}
	hosts := *cfg.Network.Hosts
	if !slices.Contains(hosts, "api.a.com") {
		t.Error("missing host api.a.com")
	}
	if !slices.Contains(hosts, "api.b.com") {
		t.Error("missing host api.b.com")
	}
}

func TestExpandHostPatterns(t *testing.T) {
	t.Parallel()

	input := []string{"oauth2.googleapis.com", "*-aiplatform.googleapis.com"}
	result := expandHostPatterns(input)

	if slices.Contains(result, "*-aiplatform.googleapis.com") {
		t.Error("registered expander should have expanded the wildcard")
	}
	if !slices.Contains(result, "us-central1-aiplatform.googleapis.com") {
		t.Error("expected expanded regional endpoint")
	}
}
