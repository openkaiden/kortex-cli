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

package openshellvm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/openkaiden/kdn/pkg/logger"
	"github.com/openkaiden/kdn/pkg/steplogger"
)

const (
	vmPIDFile           = "vm.pid"
	vmReadinessTimeout  = 5 * time.Minute
	vmReadinessInterval = 3 * time.Second
)

// isGatewayReady checks whether the OpenShell gateway is reachable by
// attempting to list sandboxes. This validates the full stack: VM running,
// k3s healthy, and gateway gRPC endpoint responding.
func (r *openshellVMRuntime) isGatewayReady(ctx context.Context) bool {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, r.executor.BinaryPath(), "sandbox", "list")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// ensureVMRunning starts the OpenShell VM if it is not already running,
// then waits for the gateway to become ready.
func (r *openshellVMRuntime) ensureVMRunning(ctx context.Context) error {
	if r.isGatewayReady(ctx) {
		return nil
	}

	step := steplogger.FromContext(ctx)
	step.Start("Starting OpenShell VM", "OpenShell VM started")

	l := logger.FromContext(ctx)
	cmd := exec.Command(r.vmBinaryPath) //nolint:gosec
	cmd.Stdout = l.Stdout()
	cmd.Stderr = l.Stderr()
	if err := cmd.Start(); err != nil {
		step.Fail(err)
		return fmt.Errorf("failed to start openshell-vm: %w", err)
	}

	// Store PID for reference
	pidPath := filepath.Join(r.storageDir, vmPIDFile)
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	// Wait for gateway readiness
	deadline := time.Now().Add(vmReadinessTimeout)
	for time.Now().Before(deadline) {
		if r.isGatewayReady(ctx) {
			return nil
		}

		select {
		case <-ctx.Done():
			step.Fail(ctx.Err())
			return ctx.Err()
		case <-time.After(vmReadinessInterval):
		}
	}

	err := fmt.Errorf("openshell gateway did not become ready within %s", vmReadinessTimeout)
	step.Fail(err)
	return err
}
