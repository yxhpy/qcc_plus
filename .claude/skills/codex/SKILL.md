---
name: codex
description: Use when the user asks to run Codex CLI (codex exec, codex resume) or references OpenAI Codex for code analysis, refactoring, or automated editing
---

# Codex Skill Guide

## Running a Task
1. **Always use model `gpt-5.1-codex-max` with reasoning effort `high`** for all Codex runs:
   - Model: `gpt-5.1-codex-max` (mandatory, do not change)
   - Reasoning effort: `high` (mandatory, do not change)
   - If the user mentions other model names (gpt-5, gpt-5-codex-max, gpt-5-codex), always use `gpt-5.1-codex-max` instead.
   - Clearly state in your summary that model `gpt-5.1-codex-max` with `high` reasoning effort was used.
2. Select the sandbox mode required for the task; default to `--sandbox read-only` for analysis-only tasks, and use `--sandbox workspace-write` when edits are needed. Only consider `--sandbox danger-full-access` when the user’s request clearly requires network or broad system access.
3. Assemble the command with the appropriate options:
   - `-m, --model <MODEL>`
   - `--config model_reasoning_effort="<high|medium|low>"`
   - `--sandbox <read-only|workspace-write|danger-full-access>`
   - `--full-auto`
   - `-C, --cd <DIR>`
   - `--skip-git-repo-check`
3. Always use --skip-git-repo-check.
4. When continuing a previous session, use `codex exec --skip-git-repo-check resume --last` via stdin. When resuming don't use any configuration flags unless explicitly requested by the user e.g. if he species the model or the reasoning effort when requesting to resume a session. Resume syntax: `echo "your prompt here" | codex exec --skip-git-repo-check resume --last 2>/dev/null`. All flags have to be inserted between exec and resume.
5. **IMPORTANT**: By default, append `2>/dev/null` to all `codex exec` commands to suppress thinking tokens (stderr). Only show stderr if the user explicitly requests to see thinking tokens or if debugging is needed.
6. Run the command, capture stdout/stderr (filtered as appropriate), and summarize the outcome for the user.
7. **After Codex completes**, inform the user: "You can resume this Codex session at any time by saying 'codex resume' or asking me to continue with additional analysis or changes."

### Quick Reference
| Use case | Sandbox mode | Key flags |
| --- | --- | --- |
| Read-only review or analysis | `read-only` | `--sandbox read-only 2>/dev/null` |
| Apply local edits | `workspace-write` | `--sandbox workspace-write --full-auto 2>/dev/null` |
| Permit network or broad access | `danger-full-access` | `--sandbox danger-full-access --full-auto 2>/dev/null` |
| Resume recent session | Inherited from original | `echo "prompt" \| codex exec --skip-git-repo-check resume --last 2>/dev/null` (no flags allowed) |
| Run from another directory | Match task needs | `-C <DIR>` plus other flags `2>/dev/null` |

## Following Up
- After every `codex` command, summarize what was done, propose concrete next steps, and, if needed, ask focused follow-up questions **only** about the task itself (not about model or reasoning-effort choices).
- When resuming, pipe the new prompt via stdin: `echo "new prompt" | codex exec resume --last 2>/dev/null`. The resumed session automatically uses the same model, reasoning effort, and sandbox mode from the original session.
- Restate that `gpt-5.1-codex-max` with `high` reasoning effort was used, along with the sandbox mode, when proposing follow-up actions.

## Error Handling
- Stop and report failures whenever `codex --version` or a `codex exec` command exits non-zero; request direction before retrying.
- Before you use high-impact flags like `--full-auto` or `--sandbox danger-full-access`, make sure the user’s request clearly implies this level of automation or access; you do not need to ask them to choose model or reasoning-effort.
- When output includes warnings or partial results, summarize them and ask how to adjust using `AskUserQuestion`.
