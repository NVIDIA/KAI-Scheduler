#!/usr/bin/env python3
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

"""
Parse and validate release notes from PR descriptions.
"""

import re
import sys
import json
from typing import Dict, List, Optional, Tuple


VALID_CATEGORIES = [
    "Added",
    "Changed",
    "Deprecated",
    "Removed",
    "Fixed",
    "Security"
]


def extract_release_notes_section(pr_body: str) -> Optional[str]:
    """
    Extract the Release Notes section from a PR body.
    
    Args:
        pr_body: The full PR body text
        
    Returns:
        The release notes section content, or None if not found
    """
    if not pr_body:
        return None
    
    # Match from "## Release Notes" to the next "## " heading (level 2) or end of string
    # (?m) enables multiline mode, ^## matches ## at start of line
    # [\s\S]*? matches any character (including newlines) non-greedily
    pattern = r'(?m)^##\s+Release\s+Notes\s+([\s\S]*?)(?=^##\s|\Z)'
    match = re.search(pattern, pr_body, re.IGNORECASE)
    
    if not match:
        return None
    
    return match.group(1).strip()


def clean_content(content: str) -> str:
    """
    Remove HTML comments and extra whitespace from content.
    
    Args:
        content: Raw content string
        
    Returns:
        Cleaned content
    """
    # Remove HTML comments
    content = re.sub(r'<!--.*?-->', '', content, flags=re.DOTALL)
    # Remove extra whitespace
    content = '\n'.join(line.strip() for line in content.split('\n'))
    # Remove multiple consecutive newlines
    content = re.sub(r'\n\s*\n', '\n', content)
    return content.strip()


def is_opt_out(content: str) -> bool:
    """
    Check if the release notes section is an opt-out (NONE).
    
    Args:
        content: The release notes content
        
    Returns:
        True if this is an opt-out
    """
    cleaned = clean_content(content)
    return cleaned.upper() == 'NONE'


def parse_release_notes(content: str) -> Dict[str, List[str]]:
    """
    Parse release notes content into categories and entries.
    
    Args:
        content: The release notes content
        
    Returns:
        Dictionary mapping category names to lists of entries
    """
    cleaned = clean_content(content)
    categories = {}
    current_category = None
    
    for line in cleaned.split('\n'):
        line = line.strip()
        if not line:
            continue
            
        # Check if this is a category heading
        category_match = re.match(r'^###\s+(.+)$', line)
        if category_match:
            category = category_match.group(1).strip()
            if category in VALID_CATEGORIES:
                current_category = category
                if current_category not in categories:
                    categories[current_category] = []
            else:
                # Reset current_category for invalid categories
                # This prevents entries under invalid categories from being
                # added to the previous valid category
                current_category = None
            continue
        
        # Check if this is a list item
        if line.startswith('-') or line.startswith('*'):
            if current_category:
                entry = line[1:].strip()
                if entry:
                    categories[current_category].append(entry)
    
    return categories


def validate_release_notes(pr_body: str) -> Tuple[bool, str, Optional[Dict[str, List[str]]]]:
    """
    Validate release notes in a PR body.
    
    Args:
        pr_body: The full PR body text
        
    Returns:
        Tuple of (is_valid, message, parsed_categories)
    """
    # Extract the release notes section
    release_notes = extract_release_notes_section(pr_body)
    
    if release_notes is None:
        return False, "Release Notes section not found in PR description", None
    
    # Check for opt-out
    if is_opt_out(release_notes):
        return True, "Release notes opted out (NONE)", None
    
    # Parse the release notes
    categories = parse_release_notes(release_notes)
    
    # Validate that we have at least one category with entries
    if not categories:
        return False, "Release notes must contain at least one valid category (Added, Changed, Deprecated, Removed, Fixed, Security) with entries, or 'NONE' to opt out", None
    
    # Check that each category has at least one entry
    empty_categories = [cat for cat, entries in categories.items() if not entries]
    if empty_categories:
        return False, f"Categories {', '.join(empty_categories)} have no entries", None
    
    return True, "Release notes are valid", categories


def format_changelog_from_prs(prs_json_path: str) -> str:
    """
    Format changelog entries from multiple PRs.
    
    Args:
        prs_json_path: Path to JSON file containing PR data
        
    Returns:
        Formatted changelog text with all entries organized by category
    """
    # Read PRs from file
    with open(prs_json_path, 'r') as f:
        prs = json.load(f)
    
    # Collect all entries by category across all PRs
    all_categories = {}
    
    for pr in prs:
        pr_number = pr['number']
        pr_url = pr['url']
        author = pr['author']
        author_url = pr['author_url']
        pr_body = pr['body']
        
        # Extract and parse release notes
        release_notes = extract_release_notes_section(pr_body)
        if release_notes is None or is_opt_out(release_notes):
            continue
        
        categories = parse_release_notes(release_notes)
        
        # Format and collect entries
        for category, entries in categories.items():
            if category not in all_categories:
                all_categories[category] = []
            
            # Add each entry with PR attribution
            for entry in entries:
                # Add PR link and author attribution if not already present
                if f"#{pr_number}" not in entry and pr_url not in entry:
                    formatted_entry = f"{entry} [#{pr_number}]({pr_url}) [{author}]({author_url})"
                else:
                    formatted_entry = entry
                
                # Avoid duplicates
                if formatted_entry not in all_categories[category]:
                    all_categories[category].append(formatted_entry)
    
    # Generate final changelog content
    category_order = ["Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"]
    changelog_lines = []
    
    for category in category_order:
        if category in all_categories and all_categories[category]:
            changelog_lines.append(f"### {category}")
            for entry in all_categories[category]:
                changelog_lines.append(f"- {entry}")
            changelog_lines.append("")
    
    return '\n'.join(changelog_lines).strip()


def main():
    """Main entry point for the script."""
    if len(sys.argv) < 2:
        print("Usage: release-notes.py <command> [args...]", file=sys.stderr)
        print("Commands:", file=sys.stderr)
        print("  validate <pr_body>        - Validate release notes", file=sys.stderr)
        print("  format <prs_json_file>    - Format changelog from multiple PRs", file=sys.stderr)
        sys.exit(1)
    
    command = sys.argv[1]
    
    if command == "validate":
        if len(sys.argv) < 3:
            print("Error: PR body required", file=sys.stderr)
            sys.exit(1)
        
        pr_body = sys.argv[2]
        is_valid, message, categories = validate_release_notes(pr_body)
        
        result = {
            "valid": is_valid,
            "message": message,
            "categories": categories
        }
        print(json.dumps(result))
        sys.exit(0 if is_valid else 1)
    
    elif command == "format":
        if len(sys.argv) < 3:
            print("Error: format requires prs_json_file", file=sys.stderr)
            sys.exit(1)
        
        prs_json_path = sys.argv[2]
        
        try:
            changelog = format_changelog_from_prs(prs_json_path)
            if changelog:
                print(changelog)
            else:
                print("No significant changes documented.")
            sys.exit(0)
        except Exception as e:
            print(f"Error: {e}", file=sys.stderr)
            sys.exit(1)
    
    else:
        print(f"Error: Unknown command '{command}'", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()


