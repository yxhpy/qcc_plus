# Git Hooks

This directory contains shared Git hooks for local branch policy enforcement and commit message validation.

## Policy

- No direct development on `test`
- No direct development on `prod`
- Commit messages must follow the conventional commit format
- Promotion flow:
  - `main -> test`
  - `test -> prod`

## Enable

Run:

```bash
git config core.hooksPath .githooks
chmod +x .githooks/commit-msg .githooks/pre-commit .githooks/pre-push
```
