# TODO: the resource-manager roadmap

adev is evolving from "scaffolder + explorer" into the universal manager of
AI coding-agent artifacts: one tool that knows every resource on the machine
(skills, plugins, marketplaces, configs), what state each is in, what is
broken or redundant, and that can act on all of it. First-class harnesses:
Claude Code, Codex, opencode.

This file is the working plan. It states why each task exists, what it
contains, and the exact protocol an agent must follow to deliver one. It is
written for the agent that picks a task: read this whole file before acting.

## How the work is organized

- **Base branch: `feat/resource-manager`**, cut from `feat/tui-base` (the
  TUI + discovery + manage work). Every task is one PR **against the base
  branch**, never against `main` or `develop`. When the roadmap completes,
  the base branch merges into `develop` through its own PR, following the
  repo's usual flow.
- **One task = one worktree = one branch = one PR.** Tasks are sized to be
  executed by independent agents in parallel, each in its own worktree, so
  nobody steps on anyone's working tree.
- **The deliverable of a task is an OPEN PR** (branch pushed, `gh pr create`
  done, checklist satisfied), not a local diff.
- The orchestrator assigns tasks and makes sure `feat/resource-manager`
  exists on origin before launching agents. Do not self-assign a task that
  another agent already has a PR or worktree for.

## Agent protocol

1. **Read `CLAUDE.md` and `PHILOSOPHY.md` first.** Honor the conventions:
   code, docs, commits and PRs in English; no em-dashes or en-dashes
   anywhere; never add `Co-Authored-By` lines to commits.
2. **Create your worktree and branch** (from the repo root):

   ```bash
   git fetch origin feat/resource-manager
   git worktree add ../adev-<task-slug> -b task/<task-slug> origin/feat/resource-manager
   cd ../adev-<task-slug>
   ```

3. **Implement.** Rules that apply to every task:
   - **TUI/CLI parity**: any operation added to the TUI ships in the same PR
     with its mirror command (with `--json` output), and vice versa.
   - Shared logic lives in `internal/` packages (`discovery`, `manage`,
     etc.), never duplicated between TUI and CLI.
   - Operations that touch Claude's plugin registry go through the claude
     CLI (`manage.Exec`), never by writing its JSON files by hand.
   - Unit tests for every new piece of logic. The pty e2e suite runs (and is
     extended) whenever the TUI changes.
4. **Verify before opening the PR.** All of these must pass:

   ```bash
   go build ./...
   go vet ./...
   gofmt -l .        # empty output
   go test ./...
   make build
   # plus the e2e suite when the TUI changed (see the recipe below)
   ```

5. **Commit in small, buildable commits** (each commit compiles on its own).
6. **Open the PR**:

   ```bash
   gh auth status                      # must be the gverdugo-dev account
   git push -u origin task/<task-slug>
   gh pr create --base feat/resource-manager --title "<imperative title>" --body "..."
   ```

   The PR body states what changed, why, and how it was verified (paste the
   test and e2e evidence). Mark your task as `in-pr` in this file inside the
   same PR.
7. **After the merge** (orchestrator or follow-up): flip the task to `done`
   here and remove the worktree with `git worktree remove ../adev-<task-slug>`.

## E2E testing recipe (pty)

The TUI must be exercised in a real pseudo-terminal. These gotchas cost real
debugging time; do not relearn them:

- Drive `expect` **event-driven**: wait for a stable on-screen marker of the
  previous state before sending the next key. Never sleep-then-send: keys
  sent before the app enters raw mode sit in the canonical buffer and arrive
  late and coalesced (esc + q becomes alt+q).
- **Answer the startup terminal queries** or the app blocks waiting for
  them: reply to OSC 11 (background color) with
  `\x1b]11;rgb:0000/0000/0000\x1b\\` and to CSI 6n (cursor position) with
  `\x1b[1;1R`.
- **Give the pty a size**: `stty rows 35 columns 120 < $spawn_out(slave,name)`;
  a 0x0 terminal renders nothing.
- **Isolate HOME**: point `$HOME` at a fixture home containing a `.claude`,
  because discovery always prepends the user's config dirs.
- `ADEV_TUI_LOG=<file>` dumps every key the program actually received,
  which is the fastest way to debug a "key did nothing" flake.

T0 turns this recipe into a committed harness. Until T0 merges, follow the
recipe by hand.

## Tasks

### T0: committed e2e harness + docs refresh  [status: in-pr]

**Why**: the pty e2e flows that verified the TUI live outside the repo, so
no one else can run or extend them; and README.md/CLAUDE.md still describe
only the scaffolder, which misleads every human and agent that reads them.

- [ ] `e2e/` folder: expect harness (query responder, readiness markers,
      sized pty, fake-home fixtures) plus a `make e2e` target.
- [ ] Port the four proven flows: root input + rescan, list scrolling,
      view navigation (tabs, drill-down, detail page), delete with confirm.
- [ ] Update README.md and CLAUDE.md: the TUI, the `scan`/`list`/`rm`/
      `plugin`/`marketplace` commands, the discovery/manage architecture,
      how to run the e2e suite.

**Deliverable**: PR whose harness runs green in a fresh clone.

### T1: install plugins + add marketplaces  [status: open]

**Why**: closes the management surface. adev can already inspect, delete,
toggle and uninstall, but the acquisition half (install a plugin, register a
marketplace) still requires using the claude CLI by hand, which breaks the
"manage everything from one place" promise.

- [ ] `manage`: `InstallPlugin(key)` (`claude plugin install`) and
      `AddMarketplace(source)` (`claude plugin marketplace add`).
- [ ] TUI: `i` on a marketplace catalog entry installs that plugin; `a` in
      the marketplaces view opens a source input form (same footer-input
      pattern as `o`); busy spinner, status line and rescan like `d`/`t`.
- [ ] CLI: `adev plugin install <name@marketplace>`,
      `adev marketplace add <source>`.
- [ ] Unit tests with a stubbed `manage.Exec`; e2e of the form UX with a
      fake `claude` script placed first in `PATH`.

**Deliverable**: PR.

### T2: duplicate detection with content hash + drift  [status: open]

**Why**: the same skill copied across N config dirs is the most common mess
in real setups, and the dangerous case is copies that diverged silently.
Grouping by name (what adev does today) cannot tell an identical copy from a
drifted one.

- [ ] `discovery`: stable content hash per artifact dir (relative paths +
      file contents, deterministic walk order).
- [ ] Groups classify locations as identical or drifted; expose which files
      differ (count is enough for v1, a diff view can come later).
- [ ] TUI: badge on group rows (`=` identical everywhere, `≠` drifted) and
      per-location hash state in the detail/preview.
- [ ] CLI: `adev list <category> --duplicates` filter; hash fields in
      `--json`.

**Deliverable**: PR.

### T3: adev doctor  [status: open]

**Why**: broken artifacts fail silently at runtime: a skill without
`SKILL.md` never triggers, a plugin enabled in settings but missing from the
registry is a ghost, a marketplace pointing at a deleted directory rots.
One prioritized report of everything wrong turns those surprises into a
checklist.

- [ ] `internal/doctor`: finding model (severity, path, message, fix hint)
      and the check runner.
- [ ] Checks: missing/invalid `SKILL.md` or frontmatter; `SKILL.md` over 200
      lines (house style); invalid `plugin.json`/`marketplace.json`; enabled
      but not installed plugins; cache entries without registry entry and
      vice versa; marketplace `directory` sources that no longer exist.
- [ ] CLI: `adev doctor [path] [--json]`.
- [ ] TUI: a fifth view (`5 doctor`) listing findings with detail pages.

**Deliverable**: PR.

### T4: adev clean  [status: open, depends: T3, benefits from T2]

**Why**: caches accumulate stale plugin versions and orphans, and doctor
findings need an actuator; deleting dozens of items one by one with `d`
does not scale.

- [ ] `internal/clean`: candidate collector (old cached plugin versions
      keeping the installed one, orphaned cache dirs, dead marketplaces,
      doctor-flagged broken artifacts).
- [ ] Dry-run by default, `--apply` to execute; in the TUI, per-candidate
      confirmation reusing the existing confirm flow.
- [ ] CLI: `adev clean [path] [--json] [--apply]`; TUI action reachable from
      the doctor view.

**Deliverable**: PR.

### T5: harness adapters  [status: open, coordinate with T2/T3]

**Why**: Claude support is deep (registry, enabled state, cache) while
codex/opencode are folder listings; the asymmetry is hardcoded. An adapter
interface makes every harness a first-class citizen and turns "support
gemini-cli/cursor" into implementing one interface instead of touching
discovery and the TUI.

- [ ] `internal/harness`: interface (marker, artifact containers, registry
      reader, manifest validators, operation executor) + adapter registry.
- [ ] Move the Claude-specific logic (registry parsing, claude CLI
      execution) into its adapter.
- [ ] Codex adapter: prompts, AGENTS.md awareness. opencode adapter:
      `opencode.json`, TS plugins.
- [ ] Discovery and TUI consume adapters only.

**Deliverable**: PR. Touches the discovery core: do not run in parallel
with T2 or T3; rebase on them instead.

### T6: adevfile manifest + sync  [status: open, depends: T1, T5]

**Why**: a machine's agent setup should be reproducible. A declarative
manifest (which skills, plugins and marketplaces you want, per harness) plus
`adev sync` generalizes the hand-written `setup.sh` pattern into the
product itself.

- [ ] Manifest schema (`adevfile`) and parser.
- [ ] `adev export`: current discovered state to an adevfile.
- [ ] `adev sync`: diff manifest vs reality, apply via manage/adapters,
      dry-run by default.

**Deliverable**: PR.

### T7: cross-harness skill install  [status: open, depends: T5, benefits from T2]

**Why**: skills are the one artifact that is portable across harnesses (the
SKILL.md standard is shared by Claude Code, Codex, opencode and 20+ agents),
but adev can only scaffold them into one harness at a time. "Put this skill
in every harness on this machine" is the natural completion of the
duplicate story: T2 detects the copies, T7 creates them on purpose.

- [ ] `internal/manage` (or the T5 adapters): `InstallSkill(srcDir, harness,
      scope)` that copies a skill directory into a harness's skills
      container, refusing to overwrite unless forced, validating SKILL.md
      frontmatter first.
- [ ] Support `--harness all` fan-out: install into every harness detected
      on the machine (both project and user scope targets).
- [ ] CLI: `adev skill install <path|discovered-name> [--harness X|all]
      [--scope user|project] [--force] [--json]`.
- [ ] TUI: `c` (copy to harness) on a skill row opens a harness picker,
      reusing the footer-input/confirm patterns; busy spinner + rescan.
- [ ] Unit tests with fixture skill dirs; e2e of the picker when the TUI
      changes.

**Deliverable**: PR.

### T8: skills.sh registry explorer  [status: open, depends: T7]

**Why**: skills.sh (Vercel Labs) is becoming the npm of agent skills: a
public directory + leaderboard with install counts across 20+ agents. adev
already manages what is on disk; the missing half is discovering and pulling
what is not. A built-in explorer closes the loop: search the registry,
inspect a skill, install it into any harness, all without leaving adev.

- [ ] `internal/registry`: client for the public JSON API
      (`GET https://www.skills.sh/api/search?q=<query>` returns
      `{skills: [{skillId, name, installs, source}]}` where `source` is a
      GitHub `owner/repo`). Verified working 2026-07-10. Keep the client
      behind an interface so other registries can plug in later.
- [ ] Skill fetch: download the source repo (shallow git clone or GitHub
      codeload tarball, no npm/npx dependency, adev stays self-contained),
      locate the skill dir by its SKILL.md, hand it to T7's InstallSkill.
- [ ] CLI: `adev search <query> [--json]` and
      `adev skill install <owner/repo/skill-id> --from-registry` (exact
      flag shape can be refined in the PR).
- [ ] TUI: a registry view (`6 explore`): search input (footer-input
      pattern), result list showing name, source and install count, detail
      page with the fetched SKILL.md preview, and `i` to install via the
      T7 harness picker.
- [ ] Unit tests with a stubbed HTTP client and a fixture tarball; never
      hit the network in tests.

**Deliverable**: PR.

## Open questions the roadmap should keep asking itself

Self-posed questions, kept here so future tasks inherit them. Promote to a
task when the answer is "yes, and it is worth a PR".

- Can we manage skill installation across ALL harnesses, not just detect
  them? (Answered: yes, that is T7.)
- Should adev browse external registries? (Answered: yes for skills.sh,
  that is T8. Others, like plugin marketplaces on GitHub, can reuse the
  registry interface.)
- Updates: once a skill is installed from skills.sh, how do we know it is
  outdated? A registry hash/version check in `adev doctor` (T3) or a
  `--check-updates` flag could cover it. Candidate T9.
- Security: third-party skills are prompts executed by your agent. Should
  T8 show a diff/preview and require explicit confirm before install
  (never auto-install)? Lean yes: preview-first is already the TUI habit.
- Drift repair: T2 detects drifted duplicates; should there be a "make all
  copies match this one" action (the write half of T2, powered by T7's
  copier)?
- Publishing: should `adev` help publish a local skill TO skills.sh
  (leaderboard indexes public GitHub repos, so this is "push to a public
  repo + register")? Probably post-v1.

## Dependency and conflict map

| Task | Depends on | Conflict-prone areas        |
| ---- | ---------- | --------------------------- |
| T0   | none       | `e2e/`, docs                |
| T1   | none       | `manage`, `cli`, `tui`      |
| T2   | none       | `discovery`, list views     |
| T3   | none       | new pkg, `cli`, new view    |
| T4   | T3 (+T2)   | new pkg, `tui`              |
| T5   | none       | `discovery` core            |
| T6   | T1, T5     | new pkg                     |
| T7   | T5 (+T2)   | `manage`/adapters, `cli`, `tui` |
| T8   | T7         | new pkg, `cli`, new view    |

Suggested waves: **wave 1** = T0, T1, T2, T3 in parallel (low overlap);
**wave 2** = T4, T5; **wave 3** = T6, T7; **wave 4** = T8. Rebase on the
base branch between waves.
