#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

set -e

CHANGELOG_FILE="${CHANGELOG_FILE:-CHANGELOG.md}"

# Function to add an entry to the [Unreleased] section
# Usage: add_entry "category" "entry text"
add_entry() {
    local category="$1"
    local entry="$2"
    
    if [ -z "$category" ] || [ -z "$entry" ]; then
        echo "Error: category and entry are required"
        return 1
    fi
    
    # Ensure the entry starts with "- "
    if [[ ! "$entry" =~ ^-\ .* ]]; then
        entry="- $entry"
    fi
    
    # Create a temporary file
    local temp_file=$(mktemp)
    
    # Track whether we found the category
    local found_category=false
    local in_unreleased=false
    local added=false
    
    while IFS= read -r line || [ -n "$line" ]; do
        echo "$line" >> "$temp_file"
        
        # Check if we're in the Unreleased section
        if [[ "$line" =~ ^##\ \[Unreleased\] ]]; then
            in_unreleased=true
            continue
        fi
        
        # Check if we've left the Unreleased section
        if [[ "$in_unreleased" == true ]] && [[ "$line" =~ ^##\ \[v.* ]]; then
            in_unreleased=false
        fi
        
        # If we're in unreleased and find the category
        if [[ "$in_unreleased" == true ]] && [[ "$line" =~ ^###\ $category$ ]]; then
            found_category=true
            # Add the entry after the category header
            echo "$entry" >> "$temp_file"
            added=true
        fi
    done < "$CHANGELOG_FILE"
    
    if [ "$added" = false ]; then
        echo "Error: Could not add entry. Category '$category' not found in [Unreleased] section."
        rm "$temp_file"
        return 1
    fi
    
    # Replace the original file
    mv "$temp_file" "$CHANGELOG_FILE"
    echo "Added entry to $category section in [Unreleased]"
}

# Function to extract the [Unreleased] section content
# Returns the content between [Unreleased] and the next version section
extract_unreleased() {
    local in_unreleased=false
    local content=""
    local has_content=false
    
    while IFS= read -r line; do
        # Start capturing after [Unreleased]
        if [[ "$line" =~ ^##\ \[Unreleased\] ]]; then
            in_unreleased=true
            continue
        fi
        
        # Stop at the next version section
        if [[ "$in_unreleased" == true ]] && [[ "$line" =~ ^##\ \[v.* ]]; then
            break
        fi
        
        # Capture content
        if [[ "$in_unreleased" == true ]]; then
            # Skip empty lines at the start
            if [ -z "$content" ] && [ -z "$line" ]; then
                continue
            fi
            
            # Check if this line has actual content (not just headers or empty)
            if [[ "$line" =~ ^-\  ]]; then
                has_content=true
            fi
            
            content="${content}${line}"$'\n'
        fi
    done < "$CHANGELOG_FILE"
    
    # Remove trailing newlines
    content=$(echo "$content" | sed -e :a -e '/^\n*$/{ $d; N; ba' -e '}')
    
    if [ "$has_content" = false ]; then
        echo "No changes"
    else
        echo "$content"
    fi
}

# Function to move [Unreleased] content to a versioned section
# Usage: move_unreleased_to_version "v0.9.2"
move_unreleased_to_version() {
    local version="$1"
    local date=$(date +%Y-%m-%d)
    
    if [ -z "$version" ]; then
        echo "Error: version is required"
        return 1
    fi
    
    # Extract unreleased content first
    local unreleased_content=$(extract_unreleased)
    
    # Create a temporary file
    local temp_file=$(mktemp)
    
    local in_unreleased=false
    local unreleased_section_written=false
    local version_section_added=false
    
    while IFS= read -r line || [ -n "$line" ]; do
        # Found [Unreleased] header
        if [[ "$line" =~ ^##\ \[Unreleased\] ]]; then
            in_unreleased=true
            # Write [Unreleased] header
            echo "$line" >> "$temp_file"
            echo "" >> "$temp_file"
            # Write empty category headers
            echo "### Added" >> "$temp_file"
            echo "" >> "$temp_file"
            echo "### Fixed" >> "$temp_file"
            echo "" >> "$temp_file"
            echo "### Changed" >> "$temp_file"
            echo "" >> "$temp_file"
            echo "### Removed" >> "$temp_file"
            unreleased_section_written=true
            continue
        fi
        
        # Skip content under [Unreleased] until we hit the next version section
        if [[ "$in_unreleased" == true ]]; then
            if [[ "$line" =~ ^##\ \[v.* ]]; then
                # We've reached the first version section, insert our new version here
                in_unreleased=false
                echo "" >> "$temp_file"
                echo "## [$version] - $date" >> "$temp_file"
                echo "" >> "$temp_file"
                echo "$unreleased_content" >> "$temp_file"
                version_section_added=true
                # Now write the line that triggered this (the existing version section)
                echo "" >> "$temp_file"
                echo "$line" >> "$temp_file"
            fi
            # Skip all other lines in unreleased section
            continue
        fi
        
        # Write all other lines
        echo "$line" >> "$temp_file"
    done < "$CHANGELOG_FILE"
    
    # If we never found a version section, add it at the end
    if [[ "$version_section_added" == false ]] && [[ "$unreleased_section_written" == true ]]; then
        echo "" >> "$temp_file"
        echo "## [$version] - $date" >> "$temp_file"
        echo "" >> "$temp_file"
        echo "$unreleased_content" >> "$temp_file"
    fi
    
    # Replace the original file
    mv "$temp_file" "$CHANGELOG_FILE"
    echo "Moved [Unreleased] content to [$version] section"
}

# Main command dispatcher
case "${1:-}" in
    add)
        add_entry "$2" "$3"
        ;;
    extract)
        extract_unreleased
        ;;
    move)
        move_unreleased_to_version "$2"
        ;;
    *)
        echo "Usage: $0 {add|extract|move}"
        echo "  add <category> <entry>  - Add entry to [Unreleased] section"
        echo "  extract                 - Extract [Unreleased] content"
        echo "  move <version>          - Move [Unreleased] to versioned section"
        exit 1
        ;;
esac

