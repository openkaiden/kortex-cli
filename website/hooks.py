#!/usr/bin/env python3
# Copyright (C) 2026 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

"""
MkDocs hooks for automatically splitting README.md into multiple pages.

This hook reads README.md and creates separate pages for each ## (level 2) heading.
The pages are created as virtual files during the build process, so README.md remains
the single source of truth.
"""

import re
import logging
from pathlib import Path

logger = logging.getLogger("mkdocs.plugins")


def slugify(text):
    """Convert heading text to a URL-friendly slug."""
    # Remove special characters and convert to lowercase
    slug = re.sub(r'[^\w\s-]', '', text.lower())
    # Replace spaces with hyphens
    slug = re.sub(r'[\s_]+', '-', slug)
    # Remove leading/trailing hyphens
    slug = slug.strip('-')
    return slug


def split_readme_content(content):
    """
    Split README.md content into sections based on ## headings.

    Returns:
        list of dict: Each dict contains 'title', 'slug', and 'content'
    """
    sections = []
    lines = content.split('\n')

    # First section: everything before first ## heading (becomes index.md)
    current_section = {
        'title': 'Home',
        'slug': 'index',
        'content': []
    }

    i = 0
    # Capture content before first ## heading
    while i < len(lines):
        line = lines[i]
        if line.startswith('## '):
            break
        current_section['content'].append(line)
        i += 1

    sections.append(current_section)

    # Process each ## section
    while i < len(lines):
        line = lines[i]

        if line.startswith('## '):
            # Start new section
            title = line[3:].strip()  # Remove "## "
            slug = slugify(title)

            current_section = {
                'title': title,
                'slug': slug,
                'content': [line]  # Include the heading
            }
            sections.append(current_section)
        else:
            # Add to current section
            current_section['content'].append(line)

        i += 1

    # Join content lines back into strings
    for section in sections:
        section['content'] = '\n'.join(section['content'])

    return sections


def on_files(files, config):
    """
    Called after files are collected from docs_dir.
    Split README.md into multiple virtual files.
    """
    from mkdocs.structure.files import File

    # Read README.md from project root
    # docs_dir is website/docs -> parent is website -> parent is repo root
    docs_dir_abs = Path(config['docs_dir']).resolve()
    readme_path = docs_dir_abs.parent.parent / 'README.md'

    if not readme_path.exists():
        logger.warning(f"README.md not found at {readme_path}")
        return files

    logger.info(f"Splitting README.md from {readme_path}")

    with open(readme_path, 'r', encoding='utf-8') as f:
        readme_content = f.read()

    # Split README into sections
    sections = split_readme_content(readme_content)

    logger.info(f"Found {len(sections)} sections in README.md")

    # Remove existing index.md symlink if it exists
    files_to_remove = [f for f in files if f.src_path == 'index.md']
    for f in files_to_remove:
        files.remove(f)

    # Create virtual files for each section
    for section in sections:
        filename = f"{section['slug']}.md"

        # Create a temporary file
        temp_path = Path(config['docs_dir']) / filename
        temp_path.write_text(section['content'], encoding='utf-8')

        # Create MkDocs File object
        file_obj = File(
            path=filename,
            src_dir=config['docs_dir'],
            dest_dir=config['site_dir'],
            use_directory_urls=config['use_directory_urls']
        )

        files.append(file_obj)
        logger.debug(f"Created virtual file: {filename} ({section['title']})")

    return files


def on_nav(nav, config, files):
    """
    Called after navigation is created.
    Build navigation structure from split sections.
    """
    from mkdocs.structure.nav import Navigation

    # Read README.md again to get section titles in order
    # Use same path calculation as on_files for consistency
    docs_dir_abs = Path(config['docs_dir']).resolve()
    readme_path = docs_dir_abs.parent.parent / 'README.md'

    if not readme_path.exists():
        return nav

    with open(readme_path, 'r', encoding='utf-8') as f:
        readme_content = f.read()

    sections = split_readme_content(readme_content)

    # Build navigation items
    nav_items = []
    for section in sections:
        filename = f"{section['slug']}.md"

        # Find corresponding file
        file_obj = None
        for f in files:
            if f.src_path == filename:
                file_obj = f
                break

        if file_obj:
            from mkdocs.structure.pages import Page
            page = Page(section['title'], file_obj, config)
            nav_items.append(page)

    # Create new navigation
    nav.items = nav_items
    nav.pages = nav_items

    return nav


def on_post_build(config):
    """
    Called after site is built.
    Clean up temporary markdown files created during build.
    """
    docs_dir = Path(config['docs_dir'])

    # Remove all .md files in docs_dir (they were created temporarily)
    for md_file in docs_dir.glob('*.md'):
        md_file.unlink()
        logger.debug(f"Cleaned up temporary file: {md_file}")
