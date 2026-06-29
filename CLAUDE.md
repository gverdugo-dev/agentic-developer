# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this is

`adev` (module `agentic-developer`) is a small Go CLI that scaffolds and removes
AI coding-agent artifacts (skills, plugins, plugin-marketplaces) for a detected
or explicit harness (Claude Code, Codex, opencode). It is self-contained: the
folder layouts and its own bundled skills are embedded in the binary at build
time, so a single binary carries everything it needs.

The opinionated approach to building skills and plugins lives in
[PHILOSOPHY.md](PHILOSOPHY.md); read it before touching the bundled skills.

## Commands

```bash
make build       # build ./bin/adev (version injected via -ldflags)
make run         # build, then run
make install     # go install ./cmd/adev
make release     # cross-compile all targets into dist/ (the release workflow runs this)
go build ./...   # compile
go vet ./...     # static checks
gofmt -l .       # formatting check (empty output = clean)
go doc ./internal/scaffolding   # browse package docs
```

There are no unit tests yet; verify changes by building and running the CLI
against a temporary HOME (see "Verifying setup" below).

## Architecture

Three layers, raw args flow down through them:

```
cmd/adev/main.go            Entry point: wires the program, owns the exit code.
internal/cli/               Boundary: parse + validate args, dispatch.
  cli.go                    ArgsBody, Run, NewArgsBody, command dispatch.
  setup.go                  `adev setup`: install bundled skills into a harness.
  version.go                `adev version` + the ldflags-injected Version var.
  update.go                 `adev update`: self-update from the latest release.
internal/scaffolding/       Domain core.
  types.go                  Typed model (AIHarness, Verb, Artifact, Scope) + Parse* validators.
  scaffold.go               Config load + ApplyConfig/RemoveConfig engine.
  utils.go                  Harness detection, placement, lookups.
  structures.json           Embedded folder layouts (//go:embed).
skills/                     Bundled skills, embedded via //go:embed.
  skills.go                 Embed + Install into a harness skills dir.
  adev-cli/, adev-skill-builder/, adev-plugin-builder/, adev-plugin-marketplace-builder/
```

### How scaffolding works

`structures.json` maps each harness to the per-artifact file/dir layout. The
destination of `adev new <artifact> <name>` is built as:

```
<scope base dir> / .<harness> / <artifact subdir> / <name>
```

Scope `project` resolves to the current dir, `local` to the user's home. Harness
detection looks only in that base dir for a marker (`.claude`, `.codex`,
`.opencode`), with priority Claude, Codex, opencode, and errors if none is found
and none is passed. `new` refuses to overwrite an existing artifact unless
`--force`.

### How setup works

`adev setup [harness]` installs adev's own bundled skills. It always targets the
user's config (home), never the project, writing to `~/.<harness>/skills/`. It
overwrites adev's own skill files (so re-running is an update) but never deletes
the skills directory or touches other skills. There is no pruning of files
removed from the bundle.

## Release flow

`make release` only builds locally into `dist/`; it does not publish. Publishing
is driven by `.github/workflows/release.yml`, which triggers on pushing a tag
`vX.Y.Z`. The workflow runs `make release VERSION=<tag>` (single source of truth
for the build matrix), then publishes the binaries and `checksums.txt` to a
GitHub Release. `install.sh` / `install.bat` download the matching asset from the
latest release, and `adev update` self-updates the same way. To cut a release:

```bash
git tag vX.Y.Z && git push origin vX.Y.Z
```

## Conventions

- **No em-dashes or en-dashes** anywhere in docs, skills, or code; use commas,
  colons, periods, or parentheses. Hyphens in identifiers (`adev-cli`) are fine.
- Code, docs, commit messages, and PRs are in English.
- Module path is `agentic-developer`; `-ldflags` set
  `agentic-developer/internal/cli.Version`.
- Never commit secrets. `bin/`, `dist/`, and `tmp/` are gitignored.
- Bundled skills follow [PHILOSOPHY.md](PHILOSOPHY.md): a `SKILL.md` index under
  200 lines, `references/` vs `assets/`, deterministic scripts, a preflight.

## Verifying setup

Run `setup` against a throwaway HOME so it does not write to your real config:

```bash
go build -o /tmp/adev ./cmd/adev
rm -rf /tmp/fh && mkdir -p /tmp/fh/.claude
HOME=/tmp/fh /tmp/adev setup
find /tmp/fh/.claude/skills -type f
```

## Known limitation

`adev new skill` cannot yet scaffold directly into a plugin's
`plugins/<plugin>/skills/` folder; it always writes to `.<harness>/skills/`. See
the README roadmap.
