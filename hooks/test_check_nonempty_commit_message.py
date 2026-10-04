"""Tests for the commit-msg guard using disposable Git repositories."""

from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().with_name("check_nonempty_commit_message.py")


class CommitMessageGuardTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary_directory = tempfile.TemporaryDirectory()
        cls.repository = Path(cls.temporary_directory.name) / "fixture repository"
        cls.repository.mkdir()
        subprocess.run(
            ["git", "init", "--quiet"],
            cwd=cls.repository,
            check=True,
        )
        cls.message_path = cls.repository / "message file with spaces.txt"

    @classmethod
    def tearDownClass(cls):
        cls.temporary_directory.cleanup()

    def run_guard(self, message, comment_char="#"):
        subprocess.run(
            ["git", "config", "core.commentChar", comment_char],
            cwd=self.repository,
            check=True,
        )
        self.message_path.write_text(message, encoding="utf-8")
        return subprocess.run(
            [sys.executable, str(SCRIPT), str(self.message_path)],
            cwd=self.repository,
            capture_output=True,
            text=True,
            check=False,
        )

    def test_accepts_a_nonempty_message_from_a_path_with_spaces(self):
        result = self.run_guard("feat(domain): add an audit record\n")

        self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_whitespace_only_message(self):
        result = self.run_guard(" \n\t\n")

        self.assertEqual(result.returncode, 1)
        self.assertIn("non-empty header", result.stderr)

    def test_rejects_comment_only_message(self):
        result = self.run_guard("# commit template hint\n# more guidance\n")

        self.assertEqual(result.returncode, 1)
        self.assertIn("non-empty header", result.stderr)

    def test_rejects_content_only_after_the_comment_scissor(self):
        result = self.run_guard(
            "# ------------------------ >8 ------------------------\n"
            "feat(domain): hidden below scissor\n"
        )

        self.assertEqual(result.returncode, 1)
        self.assertIn("non-empty header", result.stderr)

    def test_uses_a_custom_git_comment_character(self):
        result = self.run_guard("; template comment\n", comment_char=";")

        self.assertEqual(result.returncode, 1)
        self.assertIn("non-empty header", result.stderr)

    def test_rejects_gpg_signature_only_content(self):
        result = self.run_guard("gpg: signature metadata\n")

        self.assertEqual(result.returncode, 1)
        self.assertIn("non-empty header", result.stderr)


if __name__ == "__main__":
    unittest.main()
