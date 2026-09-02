# orca-autoflow Documentation

`orca-autoflow` is a Go CLI for running issue-scoped AutoFlow phases through
agent adapters. The current implementation focuses on the AutoFlow bootstrap
and phase boundary: it creates the required `.autoflow/` artifacts, composes a
phase prompt, runs Codex, validates required output artifacts, and records
Orca-owned state.

The broader pod, TUI, MCP, worktree, and shipping commands described in the
README are the product direction. This document tracks the commands that are
implemented in this repository today.

## Install From Source

```sh
git clone git@github.com:Munsik-Park/orca-autoflow.git
cd orca-autoflow
go build -o ~/go/bin/orca-autoflow ./cmd/orca
```

Confirm the binary:

```sh
orca-autoflow version
```

## Commands

```text
orca-autoflow autoflow init
orca-autoflow autoflow step
orca-autoflow version
orca-autoflow completion
```

## `autoflow init`

Create issue-scoped AutoFlow templates in a target repository.

```sh
orca-autoflow autoflow init \
  --target /path/to/project \
  --issue 123
```

The command creates these files without overwriting existing content:

```text
.autoflow/
.autoflow/issue-123-verification-design.md
.autoflow/issue-123-red-prompt.md
.autoflow/issue-123-green-prompt.md
```

Flags:

| Flag | Default | Description |
|---|---:|---|
| `--target` | `.` | Target repository root |
| `--issue` | `0` | GitHub issue number; must be positive |
| `--gitignore` | `false` | Add `.autoflow/issue-*-orca.json` to the target `.gitignore` |
| `--dry-run` | `false` | Print planned files without writing them |

Use `--gitignore` when you want Orca's local state files ignored by the target
repository:

```sh
orca-autoflow autoflow init --target /path/to/project --issue 123 --gitignore
```

## `autoflow step`

Run one artifact-gated AutoFlow phase. Supported phases are `red` and `green`.

```sh
orca-autoflow autoflow step \
  --target /path/to/project \
  --issue 123 \
  --phase red \
  --adapter codex \
  --prompt-file .autoflow/issue-123-red-prompt.md
```

The built-in Codex adapter runs:

```text
codex exec -C <target> -s <sandbox> -c approval_policy="never" ...
```

After a successful phase, `orca-autoflow` verifies that the required output
artifact exists and writes state to:

```text
.autoflow/issue-123-orca.json
```

Flags:

| Flag | Default | Description |
|---|---:|---|
| `--target` | `.` | Target repository root |
| `--issue` | `0` | GitHub issue number; must be positive |
| `--phase` | none | AutoFlow phase; currently `red` or `green` |
| `--adapter` | `codex` | Agent adapter; only `codex` is supported |
| `--model` | none | Codex model identifier; omitted means Codex default |
| `--codex-bin` | `codex` | Codex executable path; `CODEX_BIN` is also honored |
| `--profile` | none | Codex profile |
| `--sandbox` | `workspace-write` | `read-only`, `workspace-write`, or `danger-full-access` |
| `--network` | `false` | Allow network access inside Codex workspace-write sandbox |
| `--output` | none | Write Codex's last message to a file |
| `--prompt` | none | Inline task prompt |
| `--prompt-file` | none | File containing the task prompt |
| `--runner` | none | External adapter runner path |
| `--print-prompt` | `false` | Print the composed prompt without running Codex |
| `--allow-closed-issue` | `false` | Allow local replay against a closed GitHub issue |
| `--dry-run` | `false` | Validate and print command boundary without running Codex |

Exactly one prompt source should be used: `--prompt`, `--prompt-file`, or piped
stdin. `--prompt` and `--prompt-file` cannot be used together.

When `gh` and a GitHub remote are available, `autoflow step` refuses to run
against a closed issue unless `--allow-closed-issue` is set.

## Phase Artifacts

### `red`

Inputs:

```text
.autoflow/issue-123-verification-design.md
```

Outputs:

```text
.autoflow/issue-123-red.md
```

Next phase: `green`

### `green`

Inputs:

```text
.autoflow/issue-123-verification-design.md
.autoflow/issue-123-red.md
```

Outputs:

```text
.autoflow/issue-123-green.md
```

Next phase: `verify`

## External Runner Compatibility

The `--runner` flag keeps compatibility with older shell adapters:

```sh
orca-autoflow autoflow step \
  --target /path/to/project \
  --issue 123 \
  --phase red \
  --runner /path/to/codex-agent.sh \
  --prompt-file .autoflow/issue-123-red-prompt.md
```

The external runner receives target, phase, sandbox, model/profile, network, and
prompt arguments from `orca-autoflow`.

## Roadmap Surface

The public README describes the intended full Orca orchestration surface:
worktree-backed runs, pod DAGs, TUI review, MCP control, and PR shipping. Those
commands are not exposed by the current CLI yet. Until they land, use the
AutoFlow commands above as the supported entry point.
