# AI SCRM Agent Instructions

## User Context

- User: Wendy
- Role: Internet product manager
- Interest: AIGC projects
- Working philosophy: Automate or AI-enable anything repeated three times.

## Communication

- Default language: Chinese.
- Keep conclusions first, then reasons.
- Explain technical decisions with "why" and user impact, not only implementation details.
- If requirements are unclear, propose the most reasonable path first, then ask whether to adjust.
- Do not ask for confirmation unless there is real risk.

## First Principles

- Start from the essence of the problem.
- Do not copy conventions only because they are common.
- Ask: What problem is being solved? What is the most direct path? How would this be designed from zero?
- Do not flatter. Point out flawed plans directly and propose better ones when found.

## Project Setup Rules

- Before any work that changes files or external state, read both `AGENTS.md` and `MEMORY.md`.
- If either file is missing, create it before continuing.
- Fields that can be inferred should be filled automatically.
- Unknown fields should be asked in one consolidated list.
- In a workspace without both `AGENTS.md` and `MEMORY.md`, do not change files or external state except to create these two files.

## Memory Rules

- Read `MEMORY.md` at the start of project work.
- Proactively update `MEMORY.md` with new architecture decisions, pitfalls, user corrections, and external resource locations.
- Store credential locations only, never credential values.
- Do not duplicate information in `MEMORY.md` that can be found directly in code or project files.
- Before changing the structure of `AGENTS.md` or `MEMORY.md`, update the documentation first, then follow it.

## Inferred Project Context

- Project name: AI SCRM
- Initial setup date: 2026-06-24
- Apparent stack: Markdown product documents, static HTML prototype, JSON page inventories, screenshot assets.
- Workspace root: `/Users/chaoyun/Desktop/AI SCRM`
