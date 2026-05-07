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

// Package agentsetup provides centralized registration of all available agent implementations.
package agentsetup

import (
	"fmt"

	"github.com/openkaiden/kdn/pkg/agent"
)

// AgentRegistrar is an interface for types that can register agents.
// This is implemented by instances.Manager.
type AgentRegistrar interface {
	RegisterAgent(name string, ag agent.Agent) error
}

// agentFactory is a function that creates a new agent instance.
type agentFactory func() agent.Agent

// availableAgents is the list of all agents that can be registered.
// Add new agents here to make them available for automatic registration.
var availableAgents = []agentFactory{
	agent.NewClaude,
	agent.NewClaw,
	agent.NewCursor,
	agent.NewGoose,
	agent.NewOpenCode,
}

// RegisterAll registers all available agent implementations to the given registrar.
// Returns an error if any agent fails to register.
func RegisterAll(registrar AgentRegistrar) error {
	return registerAllWithFactories(registrar, availableAgents)
}

// registerAllWithFactories registers the given agents to the registrar.
// This function is internal and used for testing with custom agent lists.
func registerAllWithFactories(registrar AgentRegistrar, factories []agentFactory) error {
	for _, factory := range factories {
		ag := factory()
		if ag == nil {
			return fmt.Errorf("agent factory returned nil")
		}
		if err := registrar.RegisterAgent(ag.Name(), ag); err != nil {
			return fmt.Errorf("failed to register agent %q: %w", ag.Name(), err)
		}
	}

	return nil
}
