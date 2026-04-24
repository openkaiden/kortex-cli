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

import { OneCLI } from "@onecli-sh/sdk";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

interface Config {
  onecliUrl: string;
  gatewayUrl: string;
  apiKey: string;
  hosts: string[];
}

const CONFIG_PATH = join(__dirname, "config.json");

if (!existsSync(CONFIG_PATH)) {
  console.log("approval-handler: no config.json found, exiting (allow mode)");
  process.exit(0);
}

const config: Config = JSON.parse(readFileSync(CONFIG_PATH, "utf8"));
const allowedHosts = new Set(config.hosts ?? []);

console.log(
  `approval-handler: loaded ${allowedHosts.size} allowed host(s), connecting to ${config.onecliUrl}`,
);

const onecli = new OneCLI({
  url: config.onecliUrl,
  gatewayUrl: config.gatewayUrl,
  apiKey: config.apiKey,
});

const handle = onecli.configureManualApproval(async (request) => {
  // Strip port from host (e.g. "api.github.com:443" → "api.github.com")
  const hostname = request.host.split(":")[0];
  const decision = allowedHosts.has(hostname) ? "approve" : "deny";
  console.log(`approval-handler: ${decision} ${request.host} (hostname: ${hostname})`);
  return decision;
});

console.log("approval-handler: listening for approval requests");

// Keep the process alive — the SDK polls for approval requests but may not
// hold the event loop open on its own.
const keepalive = setInterval(() => {}, 1 << 30);

process.on("SIGTERM", () => {
  console.log("approval-handler: received SIGTERM, stopping");
  clearInterval(keepalive);
  handle.stop();
});
