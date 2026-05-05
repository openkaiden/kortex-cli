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

package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	api "github.com/openkaiden/kdn-api/cli/go"
	"github.com/openkaiden/kdn/pkg/instances"
)

// instanceToWorkspaceId converts an Instance to an api.WorkspaceId
func instanceToWorkspaceId(instance instances.Instance) api.WorkspaceId {
	return api.WorkspaceId{
		Id: instance.GetID(),
	}
}

// instanceToWorkspace converts an Instance to an api.Workspace
func instanceToWorkspace(instance instances.Instance) api.Workspace {
	runtimeData := instance.GetRuntimeData()
	ws := api.Workspace{
		Id:      instance.GetID(),
		Name:    instance.GetName(),
		Project: instance.GetProject(),
		Agent:   instance.GetAgent(),
		State:   runtimeData.State,
		Paths: api.WorkspacePaths{
			Configuration: instance.GetConfigDir(),
			Source:        instance.GetSourceDir(),
		},
		Timestamps: api.WorkspaceTimestamps{
			Created: instance.GetCreatedAt().UnixMilli(),
		},
		Forwards: []api.WorkspaceForward{},
	}
	if model := instance.GetModel(); model != "" {
		ws.Model = &model
	}
	if startedAt := instance.GetStartedAt(); !startedAt.IsZero() {
		ms := startedAt.UnixMilli()
		ws.Timestamps.Started = &ms
	}
	if forwardsJSON, ok := runtimeData.Info["forwards"]; ok {
		var forwards []api.WorkspaceForward
		if err := json.Unmarshal([]byte(forwardsJSON), &forwards); err == nil {
			ws.Forwards = forwards
		}
	}
	return ws
}

// formatErrorJSON formats an error as JSON using api.Error schema
func formatErrorJSON(err error) (string, error) {
	if err == nil {
		return "", nil
	}
	errorResponse := api.Error{
		Error: err.Error(),
	}

	jsonData, jsonErr := json.MarshalIndent(errorResponse, "", "  ")
	if jsonErr != nil {
		return "", fmt.Errorf("failed to marshal error to JSON: %w", jsonErr)
	}

	return string(jsonData), nil
}

// outputErrorIfJSON outputs the error as JSON if output mode is "json", then returns the error.
// This helper reduces code duplication for error handling in commands.
func outputErrorIfJSON(cmd interface{ OutOrStdout() io.Writer }, output string, err error) error {
	if output == "json" {
		jsonErr, _ := formatErrorJSON(err)
		fmt.Fprintln(cmd.OutOrStdout(), jsonErr)
	}
	return err
}
