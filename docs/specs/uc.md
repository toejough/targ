# Use Cases: Directory Tree Traversal (Issue #11)

## UC-1: Run ancestor targets from any subdirectory

- **Actor:** Developer at a terminal
- **Starting state:** Developer has targ target files in an ancestor directory or its `dev/` subdirectory (e.g., `~/dev/targs.go` defines a `claude` command). Developer's CWD is a descendant directory (e.g., `~/repos/myproject/src/`).
- **End state:** The `claude` command executes successfully.
- **Key interactions:** Developer runs `targ claude`. Targ walks up the directory tree from CWD, discovers `~/dev/targs.go`, compiles and runs it. No configuration or opt-in required.
- **Constraints:**
  - Discovery walks the linear ancestor path only (no sibling directories), but at each ancestor also recursively walks its `dev/` subdirectory if present.
  - Walks all the way to filesystem root (no root boundary marker).
  - Automatic — no opt-in or configuration needed.
  - Ancestor targets without a `go.mod` must still compile via isolated build (synthetic go.mod in temp/cache dir).

## UC-2: See all available targets (local, descendant, and ancestor)

- **Actor:** Developer at a terminal
- **Starting state:** Developer has targ targets defined at multiple levels of the directory tree — some in CWD or below, some in ancestor directories or their `dev/` subdirectories.
- **End state:** Developer sees a complete list of all available targets, grouped by source location.
- **Key interactions:** Developer runs `targ` (or `targ --help`). Help output shows targets from all discovered locations, with existing source attribution distinguishing where targets come from.

## UC-3: Ancestor and descendant targets coexist without conflicts

- **Actor:** Developer at a terminal
- **Starting state:** Developer has a `build` target in `~/repos/myproject/dev/` and a `build` target in `~/dev/`. Developer's CWD is `~/repos/myproject/`.
- **End state:** Targ reports a conflict error with both source locations, same as today's behavior for same-name targets from different packages.
- **Key interactions:** Developer runs `targ build`. Targ discovers both, detects the name conflict, and reports it with a suggestion to use `targ.DeregisterFrom()`.

## Discovery Model

From CWD `~/repos/personal/project/src/`:

```
~/repos/personal/project/src/**    (full subtree down — unchanged from today)
~/repos/personal/project/          (ancestor) + ~/repos/personal/project/dev/**
~/repos/personal/                  (ancestor) + ~/repos/personal/dev/**
~/repos/                           (ancestor) + ~/repos/dev/**
~/                                 (ancestor) + ~/dev/**
/                                  (ancestor) + /dev/**
```

At each ancestor: check the directory itself for targ-tagged files, plus recursively walk its `dev/` subdirectory if present. No other siblings are discovered.
