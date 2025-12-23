# Beads: The Universal Data Plane

> **Status**: Canonical Architecture Documentation
> **See also**: [pinned-beads-design.md](pinned-beads-design.md), [propulsion-principle.md](propulsion-principle.md)

## Overview

Gas Town agents coordinate through a single, unified data store: **Beads**. Every
piece of agent state - work assignments, mail messages, molecules, hooks - lives
in beads as issues with consistent field semantics.

This document explains the universal data model that powers agent coordination.

## The Core Insight

Beads is not just an issue tracker. It's a **universal data plane** where:

- **Work molecules** are issues with steps as child issues
- **Mail messages** are issues with sender/recipient encoded in fields
- **Hooks** are queries over issues (`assignee = me AND pinned = true`)
- **Inboxes** are queries over issues (`assignee = me AND status = open AND has from: label`)

Everything is an issue. The semantics come from how fields are used.

## The Unified Data Model

Every beads issue has these core fields:

| Field | Type | Purpose |
|-------|------|---------|
| `id` | string | Unique identifier (e.g., `gt-xxxx`, `hq-yyyy`) |
| `title` | string | Brief summary |
| `description` | string | Full content |
| `status` | enum | `open`, `in_progress`, `closed`, `pinned` |
| `assignee` | string | Who this is assigned to |
| `priority` | int | 0=critical, 1=high, 2=normal, 3=low, 4=backlog |
| `type` | string | `task`, `bug`, `feature`, `epic` |
| `labels` | []string | Metadata tags |
| `pinned` | bool | Whether this is pinned to assignee's hook |
| `parent` | string | Parent issue ID (for hierarchies) |
| `created_at` | timestamp | Creation time |

## Field Reuse Patterns

### Work Molecules

A **molecule** is the general abstraction for executable work in Gas Town. Molecules
can take various **shapes** - structural patterns that encode different workflow
semantics. The data plane stores all shapes the same way; the semantics come from
how the structure is interpreted.

**Common molecular shapes:**

| Shape | Structure | Use Case |
|-------|-----------|----------|
| **Epic** | Parent + flat children (TODO list) | Simple feature work, linear decomposition |
| **Christmas Ornament** | Trunk + dynamic arms + base | Patrol cycles with variable fanout |
| **Compound** | Multiple bonded molecules | Complex multi-phase workflows |
| **Polymer** | Chained compound molecules | Large-scale autonomous operation |

> **All epics are molecules. Not all molecules are epics.**
> For the full taxonomy of shapes, see [molecular-chemistry.md](molecular-chemistry.md).

Here's an **epic** (the simplest molecular shape) stored as beads data:

```
┌─────────────────────────────────────────────────────────────┐
│  Issue: gt-abc1                                             │
│  ─────────────────────────────────────────────────────────  │
│  title:       "Implement user authentication"               │
│  type:        epic                     ← SHAPE: TODO LIST   │
│  assignee:    "gastown/crew/max"                            │
│  pinned:      true                     ← ON MY HOOK         │
│  status:      in_progress                                   │
│  description: "Full auth flow with OAuth..."                │
│  children:    [gt-abc2, gt-abc3, gt-abc4]  ← FLAT CHILDREN  │
└─────────────────────────────────────────────────────────────┘
```

The hook query: `WHERE assignee = me AND pinned = true`

**Why the data plane doesn't distinguish shapes:** The shape lives in the structure
(parent/child relationships, dependency edges), not in a separate field. An epic
is recognized by its flat parent-children structure. A Christmas Ornament is
recognized by dynamic arms bonded during execution. The beads database just stores
issues with relationships - the chemistry layer interprets the shape.

### Mail Messages

Mail reuses the same fields with different semantics:

```
MAIL FIELD          →    BEADS FIELD
═══════════════════════════════════════════════════════════

To (recipient)      →    assignee
Subject             →    title
Body                →    description
From (sender)       →    labels: ["from:mayor/"]
Thread ID           →    labels: ["thread:thread-xxx"]
Reply-To            →    labels: ["reply-to:hq-yyy"]
Message Type        →    labels: ["msg-type:task"]
Unread              →    status: open
Read                →    status: closed
```

Example mail message as a bead:

```
┌─────────────────────────────────────────────────────────────┐
│  Issue: hq-def2                                             │
│  ─────────────────────────────────────────────────────────  │
│  title:       "Fix the auth bug"           ← SUBJECT        │
│  assignee:    "gastown/crew/max"           ← TO             │
│  status:      open                         ← UNREAD         │
│  labels:      ["from:mayor/",              ← FROM           │
│                "thread:thread-abc",        ← THREAD         │
│                "msg-type:task"]            ← TYPE           │
│  description: "The OAuth flow is broken..."  ← BODY         │
└─────────────────────────────────────────────────────────────┘
```

The inbox query: `WHERE assignee = me AND status = open AND has_label("from:*")`

### Distinguishing Mail from Work

How does the system know if an issue is mail or work?

| Indicator | Mail | Work |
|-----------|------|------|
| Has `from:` label | Yes | No |
| Has `pinned: true` | Rarely | Yes (when on hook) |
| Parent is molecule | No | Often |
| ID prefix | `hq-*` (town beads) | `gt-*` (rig beads) |

The `from:` label is the canonical discriminator. Regular issues don't have senders.

## Two-Tier Beads Architecture

Gas Town uses beads at two levels, each with persistent and ephemeral components:

```
══════════════════════════════════════════════════════════════════════════
                            TOWN LEVEL
══════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────┐  ┌─────────────────────────────────┐
│  PERSISTENT: ~/gt/.beads/       │  │  EPHEMERAL: ~/gt/.beads-wisp/   │
│  ─────────────────────────────  │  │  ─────────────────────────────  │
│  Prefix: hq-*                   │  │  Git tracked: NO                │
│  Git tracked: Yes               │  │  Contains:                      │
│  Contains:                      │  │    - Deacon patrol cycles       │
│    - All mail                   │  │    - Town-level ephemeral work  │
│    - Mayor coordination         │  │  Lifecycle:                     │
│    - Cross-rig work items       │  │    Created → Executed →         │
│  Sync: Direct commit to main    │  │    Squashed to digest → Deleted │
└─────────────────────────────────┘  └─────────────────────────────────┘

══════════════════════════════════════════════════════════════════════════
                             RIG LEVEL
══════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────┐  ┌─────────────────────────────────┐
│  PERSISTENT: <clone>/.beads/    │  │  EPHEMERAL: <rig>/.beads-wisp/  │
│  ─────────────────────────────  │  │  ─────────────────────────────  │
│  Prefix: gt-* (rig-specific)    │  │  Git tracked: NO                │
│  Git tracked: Yes               │  │  Contains:                      │
│  Contains:                      │  │    - Witness patrol cycles      │
│    - Project issues             │  │    - Refinery patrol cycles     │
│    - Molecules (work patterns)  │  │    - Rig-level ephemeral work   │
│    - Agent hook states          │  │  Lifecycle:                     │
│  Sync: Via beads-sync branch    │  │    Created → Executed →         │
│                                 │  │    Squashed to digest → Deleted │
└─────────────────────────────────┘  └─────────────────────────────────┘
```

Key points:
- **Each level has persistent + ephemeral storage**
- **Town persistent** (`~/gt/.beads/`) - mail, mayor work, cross-rig coordination
- **Town ephemeral** (`~/gt/.beads-wisp/`) - deacon patrols
- **Rig persistent** (`<clone>/.beads/`) - project issues, molecules, hooks
- **Rig ephemeral** (`<rig>/.beads-wisp/`) - witness/refinery patrols
- **Sync strategies**: Persistent beads sync via git; ephemeral wisps never sync

## The Query Model

Both hooks and inboxes are **views** (queries) over the flat beads collection:

```
BEADS DATABASE (flat collection)
══════════════════════════════════════════════════════════════

┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│ hq-aaa   │  │ hq-bbb   │  │ gt-xxx   │  │ gt-yyy   │
│ mail     │  │ mail     │  │ task     │  │ molecule │
│ to: max  │  │ to: joe  │  │ assign:  │  │ assign:  │
│ open     │  │ closed   │  │   max    │  │   max    │
│          │  │          │  │ pinned:  │  │          │
│ ↓ INBOX  │  │          │  │   true   │  │          │
│          │  │          │  │ ↓ HOOK   │  │          │
└──────────┘  └──────────┘  └──────────┘  └──────────┘


INBOX QUERY (for max):
  SELECT * FROM beads
  WHERE assignee = 'gastown/crew/max'
    AND status = 'open'
    AND labels CONTAINS 'from:*'
  → Returns: [hq-aaa]


HOOK QUERY (for max):
  SELECT * FROM beads
  WHERE assignee = 'gastown/crew/max'
    AND pinned = true
  → Returns: [gt-xxx]
```

There is no container. No inbox bead. No hook bead. Just queries over issues.

## Session Cycling Through the Data Lens

When an agent cycles (hands off to fresh session), the data model ensures continuity:

### What Persists (in beads)

| Data | Location | Survives Restart |
|------|----------|------------------|
| Pinned molecule | Rig beads | Yes |
| Handoff mail | Town beads | Yes |
| Issue state | Rig beads | Yes |
| Git commits | Git | Yes |

### What Doesn't Persist

| Data | Why Not |
|------|---------|
| Claude context | Cleared on restart |
| In-memory state | Process dies |
| Uncommitted changes | Not in git |
| Unflushed beads | Not synced |

### The Cycle

```
END OF SESSION                         START OF SESSION
══════════════════                     ══════════════════

1. Commit & push code                  1. gt prime (loads context)

2. bd sync (flush beads)               2. gt mol status
   └─ Molecule state saved                └─ Query: pinned = true
                                             ↓
3. gt handoff -s "..." -m "..."        3. Hook has molecule?
   └─ Creates mail in town beads          ├─ YES → Execute it
      (assignee = self)                   └─ NO  → Query inbox
                                                   ↓
4. Session dies                        4. Inbox has handoff mail?
                                          ├─ YES → Read context
                                          └─ NO  → Wait for work
```

The **molecule is the source of truth** for what you're working on.
The **handoff mail is supplementary context** (optional but helpful).

## Command to Data Mapping

| Command | Data Operation |
|---------|----------------|
| `gt mol status` | Query: `assignee = me AND pinned = true` |
| `gt mail inbox` | Query: `assignee = me AND status = open AND from:*` |
| `gt mail read X` | Read issue X, no status change |
| `gt mail delete X` | Close issue X (status → closed) |
| `gt sling X to Y` | Update X: `assignee = Y, pinned = true` |
| `bd pin X` | Update X: `pinned = true` |
| `bd close X` | Update X: `status = closed` |

## Why This Design?

### 1. Single Source of Truth

All agent state lives in beads. No separate databases, no config files, no
hidden state. If you can read beads, you can understand the entire system.

### 2. Queryable Everything

Hooks, inboxes, work queues - all are just queries. Want to find all blocked
work? Query. Want to see what's assigned to a role? Query. The data model
supports arbitrary views.

### 3. Git-Native Persistence

Beads syncs through git. This gives you:
- Version history
- Branch-based isolation
- Merge conflict resolution
- Distributed replication

### 4. No Schema Lock-in

New use cases emerge by convention, not schema changes. Mail was added by
reusing `assignee` for recipient and adding `from:` labels. No database
migration needed.

## Related Documents

- [pinned-beads-design.md](pinned-beads-design.md) - Hook semantics per role
- [propulsion-principle.md](propulsion-principle.md) - The "RUN IT" protocol
- [sling-design.md](sling-design.md) - Work assignment mechanics
- [molecular-chemistry.md](molecular-chemistry.md) - Full taxonomy of molecular shapes, phases, and operators
- [session-lifecycle.md](session-lifecycle.md) - The Single-Bond Principle and context cycling
