# AGENTS.md

## GSD Workflow

This project uses the Get Shit Done (GSD) planning workflow. Planning docs are in `.planning/`.

- **Roadmap:** `.planning/ROADMAP.md` — 5 phases, 22 requirements
- **Requirements:** `.planning/REQUIREMENTS.md` — v1 requirements with traceability
- **Project Context:** `.planning/PROJECT.md` — living project definition
- **Config:** `.planning/config.json` — YOLO mode, standard granularity, parallel execution

### Running GSD Commands

```bash
# Plan Phase 1: Regex Domainlist
/gsd-plan-phase 1

# Execute a phase
/gsd-execute-phase 1

# Review phase results
/gsd-verify-phase 1
```

## Spike Findings

- **Spike findings for bgp-dns** (implementation patterns, constraints, gotchas) → `Skill("spike-findings-bgp-dns")`
