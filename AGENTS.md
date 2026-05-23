<!-- openspdd:start -->
---
name: codex-instructions
id: codex-instructions
category: Configuration
description: Codex instruction file with SPDD workflow references
---

# SPDD Framework for Codex

This project uses the SPDD (Structured Prompt-Driven Development) methodology for AI-assisted development.

## Available Workflows

When I ask you to use SPDD workflows, refer to these templates:

### 1. REASONS-Canvas
For structured requirement analysis and prompt generation.
- **Template**: `.codex/prompts/spdd-reasons-canvas.md`
- **Usage**: "Use REASONS-Canvas to design [feature description]"
- **Output**: Structured prompt file in `spdd/prompt/` directory

### 2. SPDD Generate
For code generation from structured prompt files.
- **Template**: `.codex/prompts/spdd-generate.md`
- **Usage**: "Generate code from @spdd/prompt/[filename].md"
- **Output**: Implementation code following Operations sequence

### 3. SPDD Sync
For syncing code changes back to prompt files.
- **Template**: `.codex/prompts/spdd-sync.md`
- **Usage**: "Sync changes to @spdd/prompt/[filename].md"
- **Output**: Updated prompt file reflecting code changes

## How to Use

1. **Read the relevant template file** when I mention an SPDD workflow
2. **Follow the steps** defined in the template
3. **Apply the guardrails** specified in each template

## Quick Reference

| Workflow | When to Use | Template Location |
|----------|-------------|-------------------|
| REASONS-Canvas | Starting new feature/task | `.codex/prompts/spdd-reasons-canvas.md` |
| SPDD Generate | Implementing from prompt | `.codex/prompts/spdd-generate.md` |
| SPDD Sync | After code refactoring | `.codex/prompts/spdd-sync.md` |

<!-- openspdd:end -->
