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
| [docs/STANDARDS.md](STANDARDS.md) | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md), [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) |
| [docs/DESIGN.md](DESIGN.md) | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/JSX-RUNTIME.md](JSX-RUNTIME.md) | [docs/DESIGN.md](DESIGN.md), [docs/STANDARDS.md](STANDARDS.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) | [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
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
| [docs/APP-USAGE.md](APP-USAGE.md) | `b2811a839f7be4ce9ca85f630599081e322a115256a5fe5d43a954ade5e4dd27` | None |
| [docs/ARCHITECTURE.md](ARCHITECTURE.md) | `46c3a31b1311e010c614137298e29d18228e3dbcb738eeb7d165334472482ec9` | [docs/DATABASE.md](DATABASE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/APP-USAGE.md](APP-USAGE.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/CSS-REFERENCE.md](CSS-REFERENCE.md) | `39ba71b09fcb2f7070f4a432461430b4404354a2bc21bcb6fe3180e100e0ae8d` | None |
| [docs/DATABASE.md](DATABASE.md) | `2e876021bb7b255f3e78a01366fd7387b6e95d54b1a712a73c2beb463deacb6e` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DESIGN.md](DESIGN.md) | `64eaeea3d676e154d595a0c5a36110b9499aca6bdbfc0852e91dc5dd2bd1c9e8` | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) | `da980e7d023757bd37ca1b67f50ad1c847ff14014ea6eb3ce73d273dc513e26b` | [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/JSX-RUNTIME.md](JSX-RUNTIME.md) | `e367daaf91a8ed549ee43240c6722f710d3b98d19126bb8f998ea35907bc4238` | [docs/DESIGN.md](DESIGN.md), [docs/STANDARDS.md](STANDARDS.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | `c17820842fbfe7edba2c5f2da2291b69c36aaf03f53fd5b3687430d53abdec7d` | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | `cb7f7fea5852d90b2f00328989d55de935932220b6dddcf5566db6dec0c3624a` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/STANDARDS.md](STANDARDS.md) | `4845caae61c925b0cc23b6c9e70d7bdab7d1c7789d137b46902430323ceca45c` | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md), [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) |
| [docs/something.spec.md](something.spec.md) | `89d3ff3f68129830ea79e558f93e35e2ef461710c9cbee88b127f0c6563cd40f` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END GENERATED DOCUMENT STATE -->

## Commands

Use `./build/doccheck state check` or `make check-docs` to compare exact bytes without writing. After reviewing each changed document and the dependency guidance above, use `make docs-state-update` to acknowledge the reviewed state.
