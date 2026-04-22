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

package features

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// localFeature implements Feature for IDs beginning with "./" or "../".
type localFeature struct {
	id  string
	dir string // workspaceConfigDir
}

var _ Feature = (*localFeature)(nil)

func (f *localFeature) ID() string { return f.id }

func (f *localFeature) Download(_ context.Context, destDir string) (FeatureMetadata, error) {
	srcDir := filepath.Clean(filepath.Join(f.dir, filepath.FromSlash(f.id)))

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}

	if err := copyDir(srcDir, destDir); err != nil {
		return nil, fmt.Errorf("copying feature from %s: %w", srcDir, err)
	}

	return parseMetadata(destDir)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported in local features: %s", path)
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		if !d.Type().IsRegular() {
			return fmt.Errorf("unsupported non-regular file in local feature: %s", path)
		}

		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
