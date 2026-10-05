#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0

"""Reject blank or comment-only messages before commitlint's empty-input skip."""

from pathlib import Path
import re
import subprocess
import sys


SCISSOR = "------------------------ >8 ------------------------"


def main():
    if len(sys.argv) != 2:
        print("expected one commit-message file path", file=sys.stderr)
        return 2

    message_path = Path(sys.argv[1])
    try:
        message = message_path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as error:
        print(f"cannot read commit message {message_path}: {error}", file=sys.stderr)
        return 2

    result = subprocess.run(
        ["git", "config", "--get", "core.commentChar"],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode not in (0, 1):
        print("cannot read Git's core.commentChar setting", file=sys.stderr)
        return 2
    comment_char = result.stdout.rstrip("\r\n") if result.returncode == 0 else "#"
    if not comment_char:
        comment_char = "#"

    lines = message.splitlines()
    scissor = f"{comment_char} {SCISSOR}"
    if scissor in lines:
        lines = lines[: lines.index(scissor)]

    meaningful_lines = (
        line
        for line in lines
        if line.strip()
        and not line.startswith(comment_char)
        and not re.match(r"^\s*gpg:", line)
    )
    if next(meaningful_lines, None) is None:
        print("commit message must include a non-empty header", file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
