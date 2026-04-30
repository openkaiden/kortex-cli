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
	"fmt"
	"strings"

	"github.com/openkaiden/kdn/pkg/runtime"
)

// Terminal starts an interactive terminal session inside a running sandbox.
func (r *openshellRuntime) Terminal(ctx context.Context, instanceID string, _ string, command []string) error {
	if instanceID == "" {
		return fmt.Errorf("%w: instance ID is required", runtime.ErrInvalidParams)
	}

	if len(command) == 0 {
		args := []string{"sandbox", "connect", instanceID}
		return r.executor.RunInteractive(ctx, args...)
	}

	shellCmd := strings.Join(command, " ")
	wrappedCmd := fmt.Sprintf("source %s/.kdn-env 2>/dev/null; cd %s 2>/dev/null; exec %s", containerHome, containerWorkspaceSources, shellCmd)

	args := []string{
		"sandbox", "exec",
		"--name", instanceID,
		"--", "sh", "-c", wrappedCmd,
	}

	return r.executor.RunInteractive(ctx, args...)
}
