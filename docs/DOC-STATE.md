# Documentation State

## Purpose and limitation

This file records explicit review dependencies and SHA-256 hashes for maintained regular files recursively under `docs/`. It excludes itself and `docs/ref/`. A hash proves only that exact bytes changed after the last acknowledgement; it does not prove semantic accuracy, so reviewers must inspect the changed document and the listed dependents before running `make docs-state-update`.

The dependency table expresses review impact rather than link direction. When a source document changes, review its listed dependents for assumptions, terminology, commands, or contracts that may also need updates. `None` means no additional dependent is currently declared, not that the document can be changed without review.

## Review dependencies

<!-- BEGIN DOCUMENT REVIEW DEPENDENCIES -->

| Source document | Review dependents |
|---|---|
| [docs/ARCHITECTURE.md](ARCHITECTURE.md) | [docs/DATABASE.md](DATABASE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/APP-USAGE.md](APP-USAGE.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DATABASE.md](DATABASE.md) | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/STANDARDS.md](STANDARDS.md) | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DESIGN.md](DESIGN.md) | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/CSS-REFERENCE.md](CSS-REFERENCE.md) | None |
| [docs/APP-USAGE.md](APP-USAGE.md) | None |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/something.spec.md](something.spec.md) | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END DOCUMENT REVIEW DEPENDENCIES -->

## Acknowledged file state

<!-- BEGIN GENERATED DOCUMENT STATE -->

| Document | SHA-256 | Review dependents |
|---|---|---|
| [docs/APP-USAGE.md](APP-USAGE.md) | `bb795733c2fc59ff85f0129584690971e56ad8d143847e8cfb67d8e04a6b9f54` | None |
| [docs/ARCHITECTURE.md](ARCHITECTURE.md) | `4d0569f88b1a143c5b66f908d3d873e2b3fb3c2b1ee17157a72e482ca577af8a` | [docs/DATABASE.md](DATABASE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/APP-USAGE.md](APP-USAGE.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/CSS-REFERENCE.md](CSS-REFERENCE.md) | `f7242ba2e867a8175879a6f48edc065719c2b986f0ba8f03bdfd987f5a37ca48` | None |
| [docs/DATABASE.md](DATABASE.md) | `a2108e4e81719485d22b153824d12c7e015311e0e84ff6e33edbe498342147c7` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DESIGN.md](DESIGN.md) | `fe766b53949a31334ef60df0099872f29ca70f727dabfe2e2d448ffd5e94a7db` | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | `3f6d2e741090bfe0653d7666a70e17f3d784696e17e777c46884242ece4a2016` | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | `5f7ffc5fc5b1143a959ee07248fd65afc33e8119e4f1f760e64ee7daf95564f5` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/STANDARDS.md](STANDARDS.md) | `86d205a126eb15c6055b629b99635f405c7aa1ebb780eb6cede162a286f5f8e4` | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/something.spec.md](something.spec.md) | `89d3ff3f68129830ea79e558f93e35e2ef461710c9cbee88b127f0c6563cd40f` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END GENERATED DOCUMENT STATE -->

## Commands

Use `./build/doccheck state check` or `make check-docs` to compare exact bytes without writing. After reviewing each changed document and the dependency guidance above, use `make docs-state-update` to acknowledge the reviewed state.
