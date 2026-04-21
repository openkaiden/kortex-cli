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

package podman

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/runtime"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

// Remove removes the workspace pod and all its containers.
func (p *podmanRuntime) Remove(ctx context.Context, id string) error {
	stepLogger := steplogger.FromContext(ctx)
	defer stepLogger.Complete()

	if id == "" {
		return fmt.Errorf("%w: container ID is required", runtime.ErrInvalidParams)
	}

	// Check if the workspace container exists and get its state
	stepLogger.Start("Checking container state", "Container state checked")
	info, err := p.getContainerInfo(ctx, id)
	if err != nil {
		if isNotFoundError(err) {
			p.cleanupPodFiles(id)
			return nil
		}
		stepLogger.Fail(err)
		return err
	}

	// Check if the container is running
	if info.State == api.WorkspaceStateRunning {
		err := fmt.Errorf("container %s is still running, stop it first", id)
		stepLogger.Fail(err)
		return err
	}

	// Resolve the pod name
	podName, err := p.readPodName(id)
	if err != nil {
		return fmt.Errorf("failed to resolve pod name: %w", err)
	}

	// Remove the entire pod and all its containers
	stepLogger.Start(fmt.Sprintf("Removing pod: %s", podName), "Pod removed")
	l := logger.FromContext(ctx)
	if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "pod", "rm", "-f", podName); err != nil {
		stepLogger.Fail(err)
		return fmt.Errorf("failed to remove pod: %w", err)
	}

	p.cleanupPodFiles(id)
	p.cleanupCerts(podName)

	// Remove the container image
	imageName := info.Info["image_name"]
	if imageName != "" {
		stepLogger.Start(fmt.Sprintf("Removing image: %s", imageName), "Image removed")
		if err := p.executor.Run(ctx, l.Stdout(), l.Stderr(), "image", "rm", imageName); err != nil && !isImageNotFoundError(err) {
			stepLogger.Fail(err)
			return fmt.Errorf("failed to remove image: %w", err)
		}
	}

	return nil
}

// cleanupCerts removes the CA certificate directory for a workspace.
func (p *podmanRuntime) cleanupCerts(podName string) {
	certsDir := filepath.Join(p.storageDir, "certs", podName)
	if !strings.HasPrefix(filepath.Clean(certsDir), filepath.Join(p.storageDir, "certs")+string(filepath.Separator)) {
		return
	}
	os.RemoveAll(certsDir)
}

// isNotFoundError checks if an error indicates that a container was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "no such container") ||
		strings.Contains(errMsg, "no such object") ||
		strings.Contains(errMsg, "error getting container") ||
		strings.Contains(errMsg, "failed to inspect container")
}

// isImageNotFoundError checks if an error indicates that an image was not found.
func isImageNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "image not known") ||
		strings.Contains(errMsg, "no such image")
}
