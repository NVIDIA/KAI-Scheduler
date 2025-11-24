#!/usr/bin/env python3
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

"""
Unit tests for parse-release-notes.py
"""

import unittest
import sys
import os
import importlib.util

# Load the parse-release-notes.py module (has hyphens, can't import normally)
script_dir = os.path.dirname(os.path.abspath(__file__))
script_path = os.path.join(script_dir, 'parse-release-notes.py')

spec = importlib.util.spec_from_file_location("parse_release_notes", script_path)
prn = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prn)


class TestExtractReleaseNotesSection(unittest.TestCase):
    """Test the extract_release_notes_section function."""
    
    def test_extract_basic(self):
        """Test basic extraction of release notes section."""
        pr_body = """
## Description
This is a test PR

## Release Notes

### Fixed
- Fixed a bug

## Related Issues
Fixes #123
"""
        result = prn.extract_release_notes_section(pr_body)
        self.assertIsNotNone(result)
        self.assertIn("### Fixed", result)
        self.assertIn("Fixed a bug", result)
    
    def test_extract_missing_section(self):
        """Test when release notes section is missing."""
        pr_body = """
## Description
This is a test PR

## Related Issues
Fixes #123
"""
        result = prn.extract_release_notes_section(pr_body)
        self.assertIsNone(result)
    
    def test_extract_case_insensitive(self):
        """Test case-insensitive matching."""
        pr_body = """
## release notes

### Fixed
- Fixed a bug
"""
        result = prn.extract_release_notes_section(pr_body)
        self.assertIsNotNone(result)
        self.assertIn("Fixed a bug", result)
    
    def test_extract_stops_at_next_section(self):
        """Test that extraction stops at the next ## heading."""
        pr_body = """
## Release Notes

### Fixed
- Fixed a bug

## Additional Notes

This should not be included
"""
        result = prn.extract_release_notes_section(pr_body)
        self.assertIsNotNone(result)
        self.assertIn("Fixed a bug", result)
        self.assertNotIn("This should not be included", result)
    
    def test_extract_stops_at_next_section_no_blank_line(self):
        """Test that extraction stops at the next ## heading even without blank line."""
        pr_body = """
## Release Notes

### Fixed
- Fixed a bug
## Additional Notes

This should not be included
"""
        result = prn.extract_release_notes_section(pr_body)
        self.assertIsNotNone(result)
        self.assertIn("Fixed a bug", result)
        self.assertNotIn("This should not be included", result)


class TestCleanContent(unittest.TestCase):
    """Test the clean_content function."""
    
    def test_remove_html_comments(self):
        """Test removal of HTML comments."""
        content = """
<!-- This is a comment -->
### Fixed
- Fixed a bug
<!-- Another comment -->
"""
        result = prn.clean_content(content)
        self.assertNotIn("<!--", result)
        self.assertNotIn("comment", result)
        self.assertIn("### Fixed", result)
    
    def test_remove_extra_whitespace(self):
        """Test removal of extra whitespace."""
        content = """
### Fixed


- Fixed a bug


"""
        result = prn.clean_content(content)
        self.assertNotIn("\n\n\n", result)


class TestIsOptOut(unittest.TestCase):
    """Test the is_opt_out function."""
    
    def test_opt_out_uppercase(self):
        """Test opt-out with uppercase NONE."""
        self.assertTrue(prn.is_opt_out("NONE"))
    
    def test_opt_out_lowercase(self):
        """Test opt-out with lowercase none."""
        self.assertTrue(prn.is_opt_out("none"))
    
    def test_opt_out_mixed_case(self):
        """Test opt-out with mixed case."""
        self.assertTrue(prn.is_opt_out("NoNe"))
    
    def test_opt_out_with_whitespace(self):
        """Test opt-out with surrounding whitespace."""
        self.assertTrue(prn.is_opt_out("  NONE  "))
    
    def test_opt_out_with_comments(self):
        """Test opt-out with HTML comments."""
        self.assertTrue(prn.is_opt_out("<!-- comment --> NONE"))
    
    def test_not_opt_out(self):
        """Test that other content is not opt-out."""
        self.assertFalse(prn.is_opt_out("### Fixed\n- Fixed a bug"))


class TestParseReleaseNotes(unittest.TestCase):
    """Test the parse_release_notes function."""
    
    def test_parse_single_category(self):
        """Test parsing a single category."""
        content = """
### Fixed
- Fixed a bug
- Fixed another bug
"""
        result = prn.parse_release_notes(content)
        self.assertEqual(len(result), 1)
        self.assertIn("Fixed", result)
        self.assertEqual(len(result["Fixed"]), 2)
        self.assertIn("Fixed a bug", result["Fixed"])
        self.assertIn("Fixed another bug", result["Fixed"])
    
    def test_parse_multiple_categories(self):
        """Test parsing multiple categories."""
        content = """
### Added
- Added new feature

### Fixed
- Fixed a bug
"""
        result = prn.parse_release_notes(content)
        self.assertEqual(len(result), 2)
        self.assertIn("Added", result)
        self.assertIn("Fixed", result)
        self.assertEqual(len(result["Added"]), 1)
        self.assertEqual(len(result["Fixed"]), 1)
    
    def test_parse_with_asterisk_bullets(self):
        """Test parsing with asterisk bullets instead of dashes."""
        content = """
### Fixed
* Fixed a bug
* Fixed another bug
"""
        result = prn.parse_release_notes(content)
        self.assertEqual(len(result["Fixed"]), 2)
    
    def test_parse_ignores_invalid_categories(self):
        """Test that invalid categories are ignored."""
        content = """
### Invalid Category
- This should be ignored

### Fixed
- This should be included
"""
        result = prn.parse_release_notes(content)
        self.assertEqual(len(result), 1)
        self.assertIn("Fixed", result)
        self.assertNotIn("Invalid Category", result)
        # Verify that entries under invalid categories are not included
        self.assertEqual(len(result["Fixed"]), 1)
        self.assertNotIn("This should be ignored", result["Fixed"])
    
    def test_parse_invalid_category_resets_context(self):
        """Test that invalid categories reset the current category context."""
        content = """
### Fixed
- Valid entry 1

### Invalid Category
- Should be ignored

### Added
- Valid entry 2
"""
        result = prn.parse_release_notes(content)
        self.assertEqual(len(result), 2)
        self.assertIn("Fixed", result)
        self.assertIn("Added", result)
        
        # Verify Fixed only has its own entry
        self.assertEqual(len(result["Fixed"]), 1)
        self.assertIn("Valid entry 1", result["Fixed"])
        self.assertNotIn("Should be ignored", result["Fixed"])
        
        # Verify Added only has its own entry
        self.assertEqual(len(result["Added"]), 1)
        self.assertIn("Valid entry 2", result["Added"])
        self.assertNotIn("Should be ignored", result["Added"])
    
    def test_parse_empty_content(self):
        """Test parsing empty content."""
        result = prn.parse_release_notes("")
        self.assertEqual(len(result), 0)


class TestValidateReleaseNotes(unittest.TestCase):
    """Test the validate_release_notes function."""
    
    def test_validate_valid_notes(self):
        """Test validation of valid release notes."""
        pr_body = """
## Release Notes

### Fixed
- Fixed a bug
"""
        is_valid, message, categories = prn.validate_release_notes(pr_body)
        self.assertTrue(is_valid)
        self.assertEqual(message, "Release notes are valid")
        self.assertIsNotNone(categories)
        self.assertIn("Fixed", categories)
    
    def test_validate_missing_section(self):
        """Test validation when section is missing."""
        pr_body = """
## Description
This is a test PR
"""
        is_valid, message, categories = prn.validate_release_notes(pr_body)
        self.assertFalse(is_valid)
        self.assertIn("not found", message)
        self.assertIsNone(categories)
    
    def test_validate_opt_out(self):
        """Test validation with opt-out."""
        pr_body = """
## Release Notes

NONE
"""
        is_valid, message, categories = prn.validate_release_notes(pr_body)
        self.assertTrue(is_valid)
        self.assertIn("opted out", message)
        self.assertIsNone(categories)
    
    def test_validate_empty_notes(self):
        """Test validation with empty release notes."""
        pr_body = """
## Release Notes

Some text but no categories
"""
        is_valid, message, categories = prn.validate_release_notes(pr_body)
        self.assertFalse(is_valid)
        self.assertIn("at least one valid category", message)
    
    def test_validate_empty_category(self):
        """Test validation with empty category."""
        pr_body = """
## Release Notes

### Fixed

### Added
- Added something
"""
        is_valid, message, categories = prn.validate_release_notes(pr_body)
        self.assertFalse(is_valid)
        self.assertIn("Fixed", message)
        self.assertIn("no entries", message)


class TestFormatForChangelog(unittest.TestCase):
    """Test the format_for_changelog function."""
    
    def test_format_single_category(self):
        """Test formatting a single category."""
        categories = {
            "Fixed": ["Fixed a bug"]
        }
        result = prn.format_for_changelog(categories, 123, "https://github.com/test/repo/pull/123", "testuser", "https://github.com/testuser")
        
        self.assertIn("### Fixed", result)
        self.assertIn("Fixed a bug", result)
        self.assertIn("[#123]", result)
        self.assertIn("[testuser]", result)
    
    def test_format_multiple_categories(self):
        """Test formatting multiple categories."""
        categories = {
            "Added": ["Added feature"],
            "Fixed": ["Fixed bug"]
        }
        result = prn.format_for_changelog(categories, 123, "https://github.com/test/repo/pull/123", "testuser", "https://github.com/testuser")
        
        self.assertIn("### Added", result)
        self.assertIn("### Fixed", result)
        # Added should come before Fixed (Keep a Changelog order)
        self.assertLess(result.index("### Added"), result.index("### Fixed"))
    
    def test_format_preserves_category_order(self):
        """Test that categories are ordered according to Keep a Changelog."""
        categories = {
            "Fixed": ["Fixed bug"],
            "Added": ["Added feature"],
            "Security": ["Security fix"]
        }
        result = prn.format_for_changelog(categories, 123, "https://github.com/test/repo/pull/123", "testuser", "https://github.com/testuser")
        
        # Check order: Added, Fixed, Security
        added_pos = result.index("### Added")
        fixed_pos = result.index("### Fixed")
        security_pos = result.index("### Security")
        
        self.assertLess(added_pos, fixed_pos)
        self.assertLess(fixed_pos, security_pos)
    
    def test_format_skips_missing_categories(self):
        """Test that missing categories are skipped."""
        categories = {
            "Fixed": ["Fixed bug"]
        }
        result = prn.format_for_changelog(categories, 123, "https://github.com/test/repo/pull/123", "testuser", "https://github.com/testuser")
        
        self.assertNotIn("### Added", result)
        self.assertNotIn("### Changed", result)
    
    def test_format_doesnt_duplicate_pr_link(self):
        """Test that PR link is not duplicated if already present."""
        categories = {
            "Fixed": ["Fixed a bug [#123](https://github.com/test/repo/pull/123)"]
        }
        result = prn.format_for_changelog(categories, 123, "https://github.com/test/repo/pull/123", "testuser", "https://github.com/testuser")
        
        # Should not add PR link again
        self.assertEqual(result.count("#123"), 1)
    
    def test_format_removes_duplicate_entries(self):
        """Test that duplicate entries within a category are removed."""
        categories = {
            "Fixed": [
                "Fixed a bug",
                "Fixed another bug",
                "Fixed a bug"  # Duplicate
            ]
        }
        result = prn.format_for_changelog(categories, 123, "https://github.com/test/repo/pull/123", "testuser", "https://github.com/testuser")
        
        # Should only have 2 unique entries, not 3
        lines = [line for line in result.split('\n') if line.startswith('- ')]
        self.assertEqual(len(lines), 2, "Should have only 2 unique entries")
        
        # Verify both unique entries are present
        self.assertIn("Fixed a bug", result)
        self.assertIn("Fixed another bug", result)
        
        # Count occurrences - "Fixed a bug" should appear only once
        self.assertEqual(result.count("Fixed a bug"), 1)


class TestIntegration(unittest.TestCase):
    """Integration tests for complete workflows."""
    
    def test_full_workflow_valid_pr(self):
        """Test complete workflow with a valid PR."""
        pr_body = """## Description
This PR fixes a critical bug

## Release Notes

### Added
- Added support for custom runtime classes

### Fixed
- Fixed scheduler pod group status update conflict
- Fixed memory leak in binder

## Related Issues
Fixes #123"""
        # Validate
        is_valid, message, categories = prn.validate_release_notes(pr_body)
        self.assertTrue(is_valid)
        self.assertIsNotNone(categories)
        
        # Check that both categories were parsed
        self.assertIn("Fixed", categories, f"Categories parsed: {categories}")
        self.assertIn("Added", categories, f"Categories parsed: {categories}")
        
        # Format
        formatted = prn.format_for_changelog(categories, 123, "https://github.com/test/repo/pull/123", "testuser", "https://github.com/testuser")
        
        # Check output - Added should come before Fixed in Keep a Changelog order
        self.assertIn("### Added", formatted)
        self.assertIn("### Fixed", formatted)
        self.assertIn("custom runtime classes", formatted)
        self.assertIn("scheduler pod group", formatted)
        self.assertIn("[#123]", formatted)
        
        # Verify order: Added comes before Fixed
        added_pos = formatted.index("### Added")
        fixed_pos = formatted.index("### Fixed")
        self.assertLess(added_pos, fixed_pos, "Added should come before Fixed in changelog")
    
    def test_full_workflow_opt_out(self):
        """Test complete workflow with opt-out."""
        pr_body = """
## Description
Internal refactoring

## Release Notes

NONE

## Related Issues
None
"""
        # Validate
        is_valid, message, categories = prn.validate_release_notes(pr_body)
        self.assertTrue(is_valid)
        self.assertIsNone(categories)
        
        # Format should return empty
        release_notes = prn.extract_release_notes_section(pr_body)
        categories = prn.parse_release_notes(release_notes) if release_notes and not prn.is_opt_out(release_notes) else {}
        formatted = prn.format_for_changelog(categories, 123, "https://github.com/test/repo/pull/123", "testuser", "https://github.com/testuser")
        
        self.assertEqual(formatted, "")


def run_tests():
    """Run all tests."""
    # Create test suite
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    
    # Add all test classes
    suite.addTests(loader.loadTestsFromTestCase(TestExtractReleaseNotesSection))
    suite.addTests(loader.loadTestsFromTestCase(TestCleanContent))
    suite.addTests(loader.loadTestsFromTestCase(TestIsOptOut))
    suite.addTests(loader.loadTestsFromTestCase(TestParseReleaseNotes))
    suite.addTests(loader.loadTestsFromTestCase(TestValidateReleaseNotes))
    suite.addTests(loader.loadTestsFromTestCase(TestFormatForChangelog))
    suite.addTests(loader.loadTestsFromTestCase(TestIntegration))
    
    # Run tests
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)
    
    # Return exit code
    return 0 if result.wasSuccessful() else 1


if __name__ == "__main__":
    sys.exit(run_tests())

