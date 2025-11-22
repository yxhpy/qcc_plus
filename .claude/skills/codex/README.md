Leave a star ⭐ if you like it 😘

# Codex Integration for Claude Code

<img width="2288" height="808" alt="skillcodex" src="https://github.com/user-attachments/assets/85336a9f-4680-479e-b3fe-d6a68cadc051" />


## Purpose
Enable Claude Code to invoke the Codex CLI (`codex exec` and session resumes) for automated code analysis, refactoring, and editing workflows.

## Prerequisites
- `codex` CLI installed and available on `PATH`.
- Codex configured with valid credentials and settings.
- Confirm the installation by running `codex --version`; resolve any errors before using the skill.

## Installation

Download this repo and store the skill in ~/.claude/skills/codex

```
git clone --depth 1 git@github.com:skills-directory/skill-codex.git /tmp/skills-temp && \
mkdir -p ~/.claude/skills && \
cp -r /tmp/skills-temp/ ~/.claude/skills/codex && \
rm -rf /tmp/skills-temp
```

## Usage

### Important: Thinking Tokens
By default, this skill suppresses thinking tokens (stderr output) using `2>/dev/null` to avoid bloating Claude Code's context window. If you want to see the thinking tokens for debugging or insight into Codex's reasoning process, explicitly ask Claude to show them.

### Example Workflow

**User prompt:**
```
Use codex to analyze this repository and suggest improvements for my claude code skill.
```

**Claude Code response:**
Claude will activate the Codex skill and:
1. Use the `gpt-5.1` model for all Codex runs (if you mention `gpt-5` or `gpt-5-codex`, it will map that request to `gpt-5.1`).
2. Infer which reasoning effort level (`low`, `medium`, or `high`) is appropriate from the task size and criticality (defaulting to `medium`, and using `high` for large or high-risk refactors), without asking you to choose.
3. Select an appropriate sandbox mode (defaulting to `read-only` for analysis-only tasks, and using `workspace-write` when edits are requested).
4. Run a command like:
```bash
codex exec -m gpt-5.1 \
  --config model_reasoning_effort="high" \
  --sandbox read-only \
  --full-auto \
  --skip-git-repo-check \
  "Analyze this Claude Code skill repository comprehensively..." 2>/dev/null
```

**Result:**
Claude will summarize the Codex analysis output, highlight key suggestions, explain which model and reasoning-effort were used, and suggest concrete follow-up actions (without asking you to confirm the configuration).

### Detailed Instructions
See `SKILL.md` for complete operational instructions, CLI options, and workflow guidance.
