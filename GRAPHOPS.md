# GraphOps CUE fork

This repository is GraphOps' public fork of
[`cue-lang/cue`](https://github.com/cue-lang/cue). It carries
evaluator fixes and additions GraphOps needs before they are available
upstream. Keep upstream history intact and keep GraphOps changes in the branch
namespaces below.

## Remotes

| Remote | Repository | Purpose |
| --- | --- | --- |
| `origin` | `git@github.com:graphops/cue.git` | This fork |
| `upstream` | `https://github.com/cue-lang/cue.git` | Public upstream, fetch only |

Never push GraphOps branches or fork-only commits to `upstream`. Set its push
URL to an invalid value so it cannot happen by accident:

```bash
git remote set-url --push upstream DISABLED-never-push-to-upstream
```

## Branches

| Branch | Contents |
| --- | --- |
| `main` | Upstream `master` plus this file. It follows upstream. |
| `graphops/<upstream tag>` | An integration branch, such as `graphops/v0.18.0-rc.1`. It starts at that exact upstream tag and holds only the selected fixes and additions, plus a `fork/` directory that documents each one. This is what GraphOps builds and runs. |
| `fix/<base>-<topic>` | One fix, isolated on the same upstream tag (for example `fix/rc1-let-pushdown` on `v0.18.0-rc.1`). It's the unit we reapply to a new base or propose upstream. |

Each integration branch's `fork/README.md` lists its fixes and additions
with their confidence and tests.

## Adding a fix

1. Branch `fix/<base>-<topic>` from the upstream tag, with a regression test.
2. Validate it against the GraphOps blueprints, comparing renders with the
   current integration build.
3. Cherry-pick it onto `graphops/<upstream tag>`, add its row to
   `fork/README.md`, and push both branches.

## Moving to a new upstream version

Create `graphops/<new tag>` from the new upstream tag, then reapply each
`fix/` branch that upstream has not fixed. Drop the ones it has. Keep the
previous integration branch until nothing builds from it.

## Upstreaming and private material

This repository is public. Never commit private GraphOps code or
configuration to it: reproducers taken from private configuration, and the
analyses that quote them, are kept outside this repository. Reduce a finding
to a self-contained case before adding it here as a test.

Nothing goes upstream without approval from the fork's owners.
