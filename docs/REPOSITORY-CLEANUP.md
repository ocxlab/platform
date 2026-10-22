# Repository cleanup

Inspected and cleaned 23 October 2026, with explicit user approval. The
clearest candidates are empty remnants of the original root-level Go layout;
the maintained provider module now lives under `services/provider/`.

| Path relative to repository root | Observation | Recommendation |
| --- | --- | --- |
| `backend/` | No files, including hidden files; empty `cmd/` and `internal/` descendants; no tracked files | Removed empty tree |
| `cmd/` | No files; empty `api/` descendant; no tracked files | Removed empty tree; retained `services/provider/cmd/` |
| `internal/` | No files; empty `runtimehttp/` descendant; no tracked files | Removed empty tree; retained `services/provider/internal/` |
| `tools/` | No files; empty `check-local/` descendant; no tracked files | Removed empty tree |

No active build/configuration reference to those old root directories was found.
Absolute target paths and recursive absence of files were checked immediately
before removal. Git does not track empty directories, so this only cleaned the
local filesystem; there are no source-file deletions. Empty directories can be
recreated if needed; no content needs recovery.

## Regenerable does not mean unwanted

Ignored `.task/`, `.pnpm-store/`, frontend `node_modules/`, `.svelte-kit/`,
`.stryker-tmp/`, `test-results/` and report directories are generated or cached.
They are not dead source directories. Old test outputs can be removed after
checking that no runner is using them and retaining any evidence still needed.
Removing dependency/build caches costs subsequent setup time and offers little
structural benefit.

Do not remove `working/` wholesale. It contains demo-image working material and
native ONIX lab configuration, potentially including local keys. These need a
separate content/lifecycle review. Likewise, `assets/demo/food/` contains tracked
image assets and provenance used by demo seeding, and `services/onix/` is an
intentional integration boundary. Neither is a cleanup candidate merely because
it is not part of the Go module. Historical design/review documents retain
decisions and evidence; age alone is not a reason to discard them.

Only the four empty root trees above were removed. No user data, Docker volume,
credentials, source assets, source files or generated artifacts were removed.
