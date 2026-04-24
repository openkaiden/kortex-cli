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

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/runtime"
)

// Ensure podmanRuntime implements runtime.Dashboard at compile time.
var _ runtime.Dashboard = (*podmanRuntime)(nil)

// GetURL implements runtime.Dashboard.
// It performs a live container inspection to confirm the actual runtime state
// before reading the stored port, guarding against stale persisted state.
func (p *podmanRuntime) GetURL(ctx context.Context, instanceID string) (string, error) {
	info, err := p.getContainerInfo(ctx, instanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get container info: %w", err)
	}
	if info.State != api.WorkspaceStateRunning {
		return "", fmt.Errorf("workspace is not running")
	}
	tmplData, err := p.readPodTemplateData(instanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get dashboard URL: %w", err)
	}
	return fmt.Sprintf("http://localhost:%d", tmplData.OnecliWebPort), nil
}
