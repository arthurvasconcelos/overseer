# ROADMAP — overseer: from personal tool to multi-user tool

**Last updated:** 2026-04-07
**Status:** Draft / working plan for Arthur + collaborators

---

## Background and goal

Right now `overseer` is simultaneously two things: a generic CLI tool and Arthur's personal dotfiles/package list. These need to be separated so anyone can install the binary without getting Arthur's shell config or Brewfile baked in, and so Arthur's personal files live under his own version control — not entangled with the tool itself.

The end state:

- **overseer** — a generic CLI binary. Installable via a one-liner curl. No personal files. Source is `arthurvasconcelos/overseer`.
- **brain** — a user-owned directory (typically a private git repo) that overseer manages as the single source of truth for all personal config, dotfiles, and tooling. overseer owns the brain's structure and exposes commands to read and write it — users should not manually edit the brain unless they know what they are doing. Arthur has his; his friend has theirs.

---

## Phase A — Extract personal config from the repo

**Goal:** the overseer repo stops owning personal files. After this phase the repo compiles and all commands still work when brain path is configured — but nothing is hardcoded to Arthur's layout.

### What to do

1. **Move `dotfiles/shell/.zshrc` out** into Arthur's brain repo (e.g. `~/brain/overseer/dotfiles/shell/.zshrc`). Delete `dotfiles/` from the overseer repo.

2. **Move `Brewfile` and `Brewfile.local.example` out** into `~/brain/overseer/`. Delete them from the repo root.

3. **Update `.gitignore`** — remove `Brewfile.local` entry (moves to brain repo's gitignore).

4. **Update `cli/cmd/setup.go`** — `runSetup` hardcodes the symlink list with the zshrc path. Guard it: if source path does not exist, print `dotfiles not found — configure brain_path and run overseer brain setup` and skip rather than error. Temporary shim until Phase B.

5. **Update `cli/cmd/brew.go`** — `brewfilePath` falls back to `"Brewfile"` relative to `overseerHome`. Add graceful missing-file error pointing the user to set `brew.brewfile` in their config or set `brain_path`.

6. **Update `scripts/setup.sh`** — remove the `make_symlink` call that hardcodes the zshrc path. Binary install only. Add a comment directing users to run `overseer init` and then `overseer brain setup` after install.

7. **Update `README.md`** — remove the symlink table. Add a brief note that personal files live in the user's brain directory.

### Files changed

| File | Change |
|---|---|
| `dotfiles/shell/.zshrc` | deleted (moved to brain) |
| `Brewfile` | deleted (moved to brain) |
| `Brewfile.local.example` | deleted (moved to brain) |
| `.gitignore` | trimmed |
| `cli/cmd/setup.go` | guard symlink list with existence check |
| `cli/cmd/brew.go` | guard Brewfile path with existence check and helpful error |
| `scripts/setup.sh` | remove dotfile symlink line |
| `README.md` | update bootstrap section |

---

## Phase B — Brain directory protocol

**Goal:** overseer has a first-class concept of "the brain". Commands resolve their paths relative to it. The brain replaces the old assumption that config + dotfiles + Brewfile all live next to the binary.

### Brain directory layout convention

overseer owns the whole brain directory and is the intended interface for managing it. `brain/overseer/` holds overseer's own files; other tools can have their own sibling directories under `brain/` as overseer grows to manage them.

```
brain/
  overseer/               # overseer-specific files (managed by overseer)
    config.yaml           # portable config (integrations, git profiles, repos, etc.)
    dotfiles/             # dotfiles to be symlinked into ~/
      shell/
        .zshrc
      git/
        .gitconfig
    Brewfile              # user's Brewfile
    Brewfile.local        # machine-local packages (gitignored in brain repo)
    plugins/              # user-written overseer plugins (overseer-* binaries)
  claude/                 # example: Claude config, skills, plans (future)
  ...                     # other tools as overseer expands
```

The brain is a managed artifact — users interact with it through `overseer brain` commands. Direct edits are possible but discouraged; the structure and conventions are owned by overseer.

### Referenced vs. owned paths

Not everything overseer touches belongs inside the brain. Some external directories are **referenced by path** in `config.yaml` and remain wherever the user already has them:

- **Obsidian vaults** — vaults typically live in their own git repo, on iCloud, in Obsidian Sync, or in Dropbox. Moving or copying a vault into the brain would break the user's existing setup. overseer reads `obsidian.vault_path` from config (absolute path or `~`-relative) and treats the vault as external. `overseer brain init` and `overseer brain setup` do not touch it.
- **`overseer_home`** (managed repos workspace) — the directory where overseer clones repos is often `~/repos` or similar. It is configured separately from `brain_path` and lives outside the brain.

The rule of thumb: if users almost certainly have the directory before they ever heard of overseer, it is referenced, not owned.

### Brain path resolution (precedence)

1. `OVERSEER_BRAIN` env var
2. `system.brain_path` in `config.local.yaml`
3. `~/brain` as default

### Config changes — `cli/internal/config/config.go`

Add `BrainPath` to `SystemConfig`:

```go
type SystemConfig struct {
    GPGSSHProgram string `mapstructure:"gpg_ssh_program"`
    OverseerHome  string `mapstructure:"overseer_home"`
    BrainPath     string `mapstructure:"brain_path"`       // new
}
```

Add `BrainPath(cfg *Config) string` and `BrainOverseerPath(cfg *Config) string` resolver helpers.

### Config loading

The **primary shared config** becomes `<brain>/overseer/config.yaml`. Loading order:

1. Load `<brain>/overseer/config.yaml` (if brain path resolves and file exists)
2. Merge `~/.config/overseer/config.local.yaml` on top

Two-pass approach: load local config first (to get `brain_path`), then reload with brain config as base.

Backward compat: if no brain is configured and `~/.config/overseer/config.yaml` exists (old symlink setup), use it. This keeps existing machines working through the transition.

### Command updates

| Command | Change |
|---|---|
| `overseer brew` | `brewfilePath` falls back to `<brain>/overseer/Brewfile` |
| `overseer setup` | replace hardcoded symlink list with discovery: any file at `<brain>/overseer/dotfiles/<path>` → `~/<path>` |
| `overseer plugin` | also scan `<brain>/overseer/plugins/` automatically |
| `overseer config` | display resolved brain path and which config files were loaded |
| `overseer init` | prompt for `brain_path`, write to `config.local.yaml` |

### Note on `overseer_home` vs `brain_path`

These are two distinct concepts:
- `brain_path` — where the user's personal config/dotfiles live (e.g. `~/brain`)
- `overseer_home` — workspace root where managed repos are cloned (e.g. `~/repos/personal`)

Keep them separate. Clarify in docs.

### Files changed

| File | Change |
|---|---|
| `cli/internal/config/config.go` | `SystemConfig`, `BrainPath()`, `BrainOverseerPath()`, updated `Load()` |
| `cli/cmd/setup.go` | dotfile discovery via brain |
| `cli/cmd/brew.go` | Brewfile path via brain |
| `cli/cmd/plugin.go` | scan brain plugins dir |
| `cli/cmd/repos.go` | `resolveOverseerHome` updated |
| `cli/cmd/config.go` | show brain path in output |
| `cli/cmd/init.go` | prompt for brain_path, write to local config |

---

## Phase C — Standalone binary installer

**Goal:** anyone can install overseer with a single curl command, no repo clone required.

### What to do

1. **Create `scripts/install.sh`** — a pure binary installer extracted from `setup.sh`:
   - Detects platform
   - Fetches latest release tag from GitHub API
   - Downloads and extracts the tarball into `~/bin/`
   - Prints "run `overseer init` to configure" and exits

2. **Simplify `scripts/setup.sh`** — becomes the dev/contributor bootstrap. Calls `install.sh` for the binary, then stops. Dotfiles are the brain's responsibility.

3. **Document one-liner in README:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/arthurvasconcelos/overseer/main/scripts/install.sh | bash
   ```

4. **After install, user flow:**
   ```
   overseer init          # creates config.local.yaml, prompts for brain_path
   overseer brain init    # scaffolds brain directory (Phase D)
   overseer brain setup   # wires dotfiles + installs brew packages (Phase D)
   ```

### Files changed

| File | Change |
|---|---|
| `scripts/install.sh` | new file (binary install only) |
| `scripts/setup.sh` | simplified to dev-bootstrap |
| `README.md` | document one-liner install |

---

## Phase D — `overseer brain` subcommand

**Goal:** give users CLI commands to manage their brain directory, replacing manual steps.

### Subcommands

**`overseer brain init`**

Scaffolds a new brain directory. overseer is the authority on what the brain looks like:
- Resolves brain path from config (or prompts if not set)
- Creates the full `<brain>/` structure: `overseer/config.yaml` (commented example), `overseer/dotfiles/shell/`, `overseer/dotfiles/git/`, `overseer/plugins/`, `overseer/Brewfile`, `overseer/Brewfile.local.example`
- Does not overwrite existing files
- Prints next steps

**`overseer brain setup`**

Replaces the dotfiles + brew half of `overseer setup`:
- Reads dotfiles from `<brain>/overseer/dotfiles/` and creates symlinks
- On macOS: runs `brew bundle install` against `<brain>/overseer/Brewfile`
- Supports `--dry-run`

**`overseer brain status`**

Shows brain path, config.yaml existence, dotfiles wired count, Brewfile presence. Useful for debugging.

**`overseer brain path`**

Prints the resolved brain path. Handy for scripts.

### Implementation notes

- New file `cli/cmd/brain.go` — cobra parent with subcommands.
- `overseer setup` becomes a thin alias for `overseer brain setup` with a deprecation notice.
- Extract the dotfile discovery + symlink logic into a shared helper to avoid duplication between `setup.go` and `brain.go`.

### Files changed

| File | Change |
|---|---|
| `cli/cmd/brain.go` | new file with all brain subcommands |
| `cli/cmd/setup.go` | symlink logic extracted to helper; becomes deprecated shim |
| `cli/internal/symlink/symlink.go` | add `MakeAll(dotfilesDir, homeDir string, dryRun bool)` helper |

---

## Phase E — Documentation and example brain

**Goal:** a new user can go from zero to working setup by reading the README.

### What to do

1. **Rewrite `README.md`** — restructure around the new model:
   - What is overseer (the tool)
   - What is a brain (the user's personal config repo)
   - Install: one-liner curl
   - First-time setup: `overseer init` → `overseer brain init` → `overseer brain setup`
   - Brain layout reference
   - Config reference: all `config.yaml` keys with examples
   - Plugin system
   - For contributors: build from source

2. **Add `brain-example/` directory** to the repo — shows the full brain layout overseer expects and manages:
   ```
   brain-example/
     overseer/
       config.yaml.example         # fully commented, all keys shown
       dotfiles/
         shell/
           .zshrc.example          # minimal starter with overseer PATH setup
       Brewfile.example
       Brewfile.local.example
       plugins/
         README.md                 # how to write a plugin
   ```
   The top-level `brain-example/` (not just `brain-example/overseer/`) is the reference layout, reinforcing that overseer owns the whole brain.

3. **`overseer brain init`** templates from these example files when scaffolding.

### Files changed

| File | Change |
|---|---|
| `README.md` | full rewrite |
| `brain-example/` | new directory |

---

## Sequencing and dependencies

```
Phase A  ──►  Phase B  ──►  Phase D
                 │
                 ▼
             Phase C  ──►  Phase E
```

Phase A can be done immediately — mostly deletion and guards. Phase B is the core architectural work; do it before D. Phase C is largely independent of B/D but benefits from the `brain_path` prompt added to `overseer init` in Phase B. Phase E is last.

---

## Decisions

1. **Brain config loading backward compat** — not needed. This project is still in active development with no production users. Phase B will hard-switch to brain-path-based loading; the old symlink at `~/.config/overseer/config.yaml` is removed as part of Arthur's own migration.

2. **`overseer setup` deprecation timeline** — not needed for the same reason. `overseer setup` is replaced directly by `overseer brain setup` in Phase D with no transition period.

3. **Multi-brain / team brains** — out of scope for now. The `brain_path` abstraction makes it straightforward to add later (e.g. `overseer brain switch <name>`, multiple named brain profiles).
