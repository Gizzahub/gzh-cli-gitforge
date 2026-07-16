# 2. Architectural Overview

> gzh-cli-gitforge 아키텍처 문서 · [인덱스](README.md) · [ARCHITECTURE.md](../../ARCHITECTURE.md)

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    gzh-cli-gitforge System                        │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌────────────────────────────────────────────────────┐     │
│  │              CLI Layer (cmd/)                       │     │
│  │  ┌──────────┐  ┌─────────┐  ┌──────────┐          │     │
│  │  │ Commands │  │ Output  │  │    UI    │          │     │
│  │  │  (Cobra) │  │ Format  │  │ Progress │          │     │
│  │  └────┬─────┘  └────┬────┘  └─────┬────┘          │     │
│  └───────┼─────────────┼──────────────┼───────────────┘     │
│          │             │              │                      │
│  ┌───────┼─────────────┼──────────────┼───────────────┐     │
│  │       ▼             ▼              ▼               │     │
│  │          Public Library API (pkg/)                 │     │
│  │  ┌──────────┐  ┌────────┐  ┌────────┐  ┌────────┐ │     │
│  │  │Repository│  │ Branch │  │ History│  │ Merge  │ │     │
│  │  │  Client  │  │Manager │  │Analyzer│  │Manager │ │     │
│  │  └────┬─────┘  └───┬────┘  └───┬────┘  └───┬────┘ │     │
│  └───────┼────────────┼───────────┼───────────┼──────┘     │
│          │            │           │           │             │
│  ┌───────┼────────────┼───────────┼───────────┼──────┐     │
│  │       ▼            ▼           ▼           ▼      │     │
│  │      Internal Implementation (internal/)          │     │
│  │  ┌─────────┐  ┌─────────┐  ┌────────────┐        │     │
│  │  │ Git CMD │  │ Parsers │  │ Validation │        │     │
│  │  │Executor │  │ (status,│  │  (input)   │        │     │
│  │  │         │  │log,diff)│  │            │        │     │
│  │  └────┬────┘  └────┬────┘  └─────┬──────┘        │     │
│  └───────┼────────────┼──────────────┼───────────────┘     │
│          │            │              │                      │
│  ┌───────┼────────────┼──────────────┼───────────────┐     │
│  │       ▼            ▼              ▼               │     │
│  │              External Dependencies                │     │
│  │  ┌─────────┐  ┌────────────┐  ┌──────────┐      │     │
│  │  │ Git CLI │  │ Filesystem │  │   OS     │      │     │
│  │  │ (2.30+) │  │   (I/O)    │  │(exec,env)│      │     │
│  │  └─────────┘  └────────────┘  └──────────┘      │     │
│  └───────────────────────────────────────────────────┘     │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Layer Responsibilities

**CLI Layer (`cmd/`):**

- User interaction (prompts, confirmations)
- Command parsing (Cobra)
- Output formatting (table, JSON)
- Progress reporting
- Configuration file management

**Library Layer (`pkg/`):**

- Core Git operations
- Business logic (commit automation, conflict resolution)
- Public APIs for external consumers
- NO CLI dependencies (no Cobra, no fmt.Println)

**Internal Layer (`internal/`):**

- Git command execution
- Output parsing (status, log, diff)
- Input validation and sanitization
- Shared utilities (not exposed)

**External Layer:**

- Git CLI binary (system installation)
- Filesystem I/O
- Operating system (exec, environment)
