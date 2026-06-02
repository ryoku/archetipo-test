---
name: pr-review
description: Critically reviews comments in a PR reviews and fixes relevant ones.
---

Use the /receiving-code-review skill to review comments to GitHub pull request identified by its number.
Consider only comments that are neither Outdated nor Resolved.
Fix relevant issues one by one, using tdd when applicable. One commit per fixed issue.

Post a reply to each comment, irrespective if it is fixed or pushed back.

Use gh cli to access GitHub.
On Windows use Git Bash.
