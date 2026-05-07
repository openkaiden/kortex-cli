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

package credentialsetup

import (
	"testing"

	"github.com/openkaiden/kdn/pkg/credential"
)

type fakeRegistrar struct {
	registered []string
}

func (f *fakeRegistrar) RegisterCredential(c credential.Credential) error {
	f.registered = append(f.registered, c.Name())
	return nil
}

func TestRegisterAll(t *testing.T) {
	t.Parallel()

	r := &fakeRegistrar{}
	if err := RegisterAll(r); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}

	if len(r.registered) == 0 {
		t.Fatal("RegisterAll() registered no credentials")
	}

	for _, want := range []string{"gcloud", "kubeconfig"} {
		found := false
		for _, name := range r.registered {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RegisterAll() did not register %q; got %v", want, r.registered)
		}
	}
}
