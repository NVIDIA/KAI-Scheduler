#!/usr/bin/env python3
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

"""
Update CHANGELOG.md with a new version section in correct semver order.
"""

import sys
import re
from packaging.version import Version


def update_changelog(changelog_path: str, version: str, date: str, entries_path: str) -> None:
    """
    Update CHANGELOG.md with a new version section.
    
    Args:
        changelog_path: Path to CHANGELOG.md file
        version: Version string (e.g., 'v0.9.3')
        date: Date string (e.g., '2025-01-15')
        entries_path: Path to file containing changelog entries
    """
    # Read existing changelog
    with open(changelog_path, 'r') as f:
        content = f.read()
    
    # Read new entries
    with open(entries_path, 'r') as f:
        changelog_entries = f.read().strip()
    
    # Create new version section
    # Add one blank line after the header, and one blank line after the content
    new_section = f"## [{version}] - {date}\n\n{changelog_entries}\n\n"
    
    # Find all existing version sections
    version_pattern = r'## \[([^\]]+)\] - \d{4}-\d{2}-\d{2}'
    matches = list(re.finditer(version_pattern, content))
    
    # Find the correct position to insert the new version
    insert_position = None
    
    # Start after the Unreleased section
    unreleased_match = re.search(r'## \[Unreleased\]\s*\n', content)
    if unreleased_match:
        search_start = unreleased_match.end()
    else:
        search_start = 0
    
    # Parse the new version using packaging.version.Version
    new_version_obj = Version(version.lstrip('v'))
    
    # Find where to insert based on semver ordering (descending)
    for match in matches:
        existing_version_str = match.group(1)
        
        # Skip if this match is before our search start
        if match.start() < search_start:
            continue
        
        # Parse existing version using packaging.version.Version
        try:
            existing_version_obj = Version(existing_version_str.lstrip('v'))
            
            # Compare versions (descending order - newer versions first)
            if new_version_obj > existing_version_obj:
                # New version is greater, insert before this one
                insert_position = match.start()
                break
        except Exception:
            # Skip invalid version strings
            continue
    
    # Insert the new version section
    if insert_position is not None:
        # Insert before the found position
        updated_content = content[:insert_position] + new_section + content[insert_position:]
    elif unreleased_match:
        # No existing version is smaller, insert right after Unreleased
        insert_position = unreleased_match.end()
        # Add a newline after Unreleased if not present
        if content[insert_position:insert_position+1] != '\n':
            updated_content = content[:insert_position] + '\n' + new_section + content[insert_position:]
        else:
            updated_content = content[:insert_position] + new_section + content[insert_position:]
    else:
        # No Unreleased section, insert at the beginning
        updated_content = new_section + content
    
    # Write updated changelog
    with open(changelog_path, 'w') as f:
        f.write(updated_content)
    
    print(f"✅ Updated {changelog_path} with {version} in correct semver order")


def main():
    """Main entry point."""
    if len(sys.argv) != 5:
        print("Usage: update-changelog.py <changelog_path> <version> <date> <entries_path>", file=sys.stderr)
        print("Example: update-changelog.py CHANGELOG.md v0.9.3 2025-01-15 changelog_entries.txt", file=sys.stderr)
        sys.exit(1)
    
    changelog_path = sys.argv[1]
    version = sys.argv[2]
    date = sys.argv[3]
    entries_path = sys.argv[4]
    
    try:
        update_changelog(changelog_path, version, date, entries_path)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()

