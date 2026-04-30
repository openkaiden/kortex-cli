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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/openkaiden/kdn/pkg/steplogger"
)

const (
	gatewayPIDFile           = "gateway.pid"
	gatewayLogFile           = "gateway.log"
	gatewayReadinessTimeout  = 5 * time.Minute
	gatewayReadinessInterval = 3 * time.Second
	gatewayPort              = "8080"
	gatewayURL               = "http://127.0.0.1:" + gatewayPort
	supervisorImage          = "quay.io/fbenoit/openshell-supervisor:2026-04-29"
)

// isGatewayReady checks whether the OpenShell gateway is reachable by
// attempting to list sandboxes. This validates the full stack: gateway running
// and responding to sandbox operations.
func (r *openshellRuntime) isGatewayReady(ctx context.Context) bool {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, r.executor.BinaryPath(), "sandbox", "list")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// ensureGatewayRunning starts the OpenShell Gateway if it is not already running,
// then waits for the gateway to become ready.
func (r *openshellRuntime) ensureGatewayRunning(ctx context.Context) error {
	// Always ensure the gateway is registered with the CLI, even if it's
	// already running (the CLI config may have been lost across restarts).
	_ = r.ensureGatewayRegistered(ctx)

	if r.isGatewayReady(ctx) {
		return nil
	}

	step := steplogger.FromContext(ctx)
	step.Start("Starting OpenShell Gateway", "OpenShell Gateway started")

	sshSecret, err := generateSSHSecret()
	if err != nil {
		step.Fail(err)
		return err
	}

	// Redirect gateway output to a log file. The gateway is a long-running
	// background process that outlives the kdn command, so piping to the
	// command's stdout/stderr would leak output to the terminal.
	logFile, err := os.OpenFile(
		filepath.Join(r.storageDir, gatewayLogFile),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644,
	)
	if err != nil {
		step.Fail(err)
		return fmt.Errorf("failed to create gateway log file: %w", err)
	}

	cmd, err := r.buildGatewayCommand(sshSecret)
	if err != nil {
		logFile.Close()
		step.Fail(err)
		return err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		step.Fail(err)
		return fmt.Errorf("failed to start openshell-gateway: %w", err)
	}

	// Monitor the process in the background so we can detect early exits.
	processExited := make(chan error, 1)
	go func() {
		processExited <- cmd.Wait()
		logFile.Close()
	}()

	// Store PID for reference
	pidPath := filepath.Join(r.storageDir, gatewayPIDFile)
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	// Register the gateway with the CLI before polling readiness.
	// The readiness check uses "openshell sandbox list" which requires
	// the gateway to be registered first.
	if err := r.ensureGatewayRegistered(ctx); err != nil {
		step.Fail(err)
		return fmt.Errorf("failed to register gateway: %w", err)
	}

	// Wait for gateway readiness
	deadline := time.Now().Add(gatewayReadinessTimeout)
	for time.Now().Before(deadline) {
		select {
		case exitErr := <-processExited:
			err := r.gatewayExitError(exitErr)
			step.Fail(err)
			return err
		default:
		}

		if r.isGatewayReady(ctx) {
			return nil
		}

		select {
		case exitErr := <-processExited:
			err := r.gatewayExitError(exitErr)
			step.Fail(err)
			return err
		case <-ctx.Done():
			step.Fail(ctx.Err())
			return ctx.Err()
		case <-time.After(gatewayReadinessInterval):
		}
	}

	err = fmt.Errorf("openshell gateway did not become ready within %s", gatewayReadinessTimeout)
	step.Fail(err)
	return err
}

// buildGatewayCommand constructs the exec.Cmd for starting the gateway
// based on the configured driver (podman or vm).
func (r *openshellRuntime) buildGatewayCommand(sshSecret string) (*exec.Cmd, error) {
	switch r.config.Driver {
	case DriverVM:
		return r.buildVMGatewayCommand(sshSecret), nil
	case DriverPodman, "":
		return r.buildPodmanGatewayCommand(sshSecret), nil
	default:
		return nil, fmt.Errorf("unsupported gateway driver: %s (supported: podman, vm)", r.config.Driver)
	}
}

func (r *openshellRuntime) buildPodmanGatewayCommand(sshSecret string) *exec.Cmd {
	dbPath := filepath.Join(r.storageDir, "openshell-podman.db")

	cmd := exec.Command(r.gatewayBinaryPath, //nolint:gosec
		"--drivers", "podman",
		"--port", gatewayPort,
		"--db-url", fmt.Sprintf("sqlite:%s", dbPath),
		"--disable-tls",
	)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OPENSHELL_SUPERVISOR_IMAGE=%s", supervisorImage),
		fmt.Sprintf("OPENSHELL_SSH_HANDSHAKE_SECRET=%s", sshSecret),
	)
	return cmd
}

func (r *openshellRuntime) buildVMGatewayCommand(sshSecret string) *exec.Cmd {
	dbPath := filepath.Join(r.storageDir, "openshell-vm.db")
	binDir := filepath.Join(r.storageDir, "bin")
	vmStateDir := filepath.Join(r.storageDir, "vm-driver")

	cmd := exec.Command(r.gatewayBinaryPath, //nolint:gosec
		"--drivers", "vm",
		"--port", gatewayPort,
		"--db-url", fmt.Sprintf("sqlite:%s", dbPath),
		"--driver-dir", binDir,
		"--grpc-endpoint", gatewayURL,
		"--ssh-handshake-secret", sshSecret,
		"--disable-tls",
	)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OPENSHELL_VM_DRIVER_STATE_DIR=%s", vmStateDir),
	)
	return cmd
}

// gatewayExitError builds an error message when the gateway process exits unexpectedly,
// including the last lines from the gateway log file for diagnostics.
func (r *openshellRuntime) gatewayExitError(exitErr error) error {
	logPath := filepath.Join(r.storageDir, gatewayLogFile)
	logData, readErr := os.ReadFile(logPath)
	if readErr != nil || len(logData) == 0 {
		return fmt.Errorf("openshell-gateway process exited unexpectedly: %w", exitErr)
	}

	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	const maxLines = 20
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	tail := strings.Join(lines, "\n")
	return fmt.Errorf("openshell-gateway process exited unexpectedly: %w\ngateway log:\n%s", exitErr, tail)
}

// ensureGatewayRegistered registers the gateway with the openshell CLI if not already registered.
func (r *openshellRuntime) ensureGatewayRegistered(ctx context.Context) error {
	// Discard stdout/stderr: the "already exists" error is expected and
	// handled below, so we don't want it leaking to the user's terminal.
	err := r.executor.Run(ctx, io.Discard, io.Discard,
		"gateway", "add", gatewayURL, "--local",
	)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// generateSSHSecret generates a random 16-byte hex-encoded SSH handshake secret.
func generateSSHSecret() (string, error) {
	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", fmt.Errorf("failed to generate SSH secret: %w", err)
	}
	return hex.EncodeToString(secretBytes), nil
}
