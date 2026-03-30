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

import "context"

type contextKey struct{}

// WithLogger returns a new context with the logger attached.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the logger from context.
// Returns a NoOpLogger if no logger is in context.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(contextKey{}).(Logger); ok {
		return l
	}
	return NewNoOpLogger()
}
