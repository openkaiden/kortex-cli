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

package fake

import (
	"context"

	"github.com/openkaiden/kdn/pkg/runtime"
)

// runtimeWithDashboard wraps fakeRuntime and adds Dashboard support.
type runtimeWithDashboard struct {
	*fakeRuntime
	url string
}

// Ensure runtimeWithDashboard implements runtime.Dashboard at compile time.
var _ runtime.Dashboard = (*runtimeWithDashboard)(nil)

// NewWithDashboard creates a fake runtime that implements the Dashboard interface.
// The url parameter is returned by GetURL for all instances.
func NewWithDashboard(url string) runtime.Runtime {
	return &runtimeWithDashboard{
		fakeRuntime: New().(*fakeRuntime),
		url:         url,
	}
}

// GetURL implements runtime.Dashboard.
func (r *runtimeWithDashboard) GetURL(_ context.Context, _ string) (string, error) {
	return r.url, nil
}
