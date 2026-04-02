# Git Hooks

This directory contains shared Git hooks for local branch policy enforcement.

## Policy

- No direct development on `test`
- No direct development on `prod`
- Promotion flow:
  - `main -> test`
  - `test -> prod`

## Enable

Run:

```bash
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit .githooks/pre-push
```
