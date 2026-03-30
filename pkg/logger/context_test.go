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

package logger

import (
	"bytes"
	"context"
	"testing"
)

func TestWithLogger_AndFromContext(t *testing.T) {
	t.Parallel()

	l := NewTextLogger(&bytes.Buffer{}, &bytes.Buffer{})

	ctx := WithLogger(context.Background(), l)

	retrieved := FromContext(ctx)

	if retrieved != l {
		t.Error("Expected to retrieve the same logger from context")
	}
}

func TestFromContext_NoLogger(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	retrieved := FromContext(ctx)

	if _, ok := retrieved.(*noopLogger); !ok {
		t.Error("Expected noopLogger when context has no logger")
	}
}

func TestFromContext_WrongType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), contextKey{}, "not a logger")

	retrieved := FromContext(ctx)

	if _, ok := retrieved.(*noopLogger); !ok {
		t.Error("Expected noopLogger when context value is wrong type")
	}
}
