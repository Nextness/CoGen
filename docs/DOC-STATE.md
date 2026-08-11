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
| [docs/APP-USAGE.md](APP-USAGE.md) | `eeead585810c095b790f84238a8beaf7b734170b72c68c0ba543c806be9f5560` | None |
| [docs/ARCHITECTURE.md](ARCHITECTURE.md) | `8fe9384f063426b728619bd2506480e17379159eb255c4f19d958412026ed953` | [docs/DATABASE.md](DATABASE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/APP-USAGE.md](APP-USAGE.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/CSS-REFERENCE.md](CSS-REFERENCE.md) | `3f129709644a45dc60d1c2ceffb2f5d0e640f5a3012a2ef435ece4a492127ad1` | None |
| [docs/DATABASE.md](DATABASE.md) | `65388c3144690663276c0e892d7471f2d3610986bd7ef9e9de48173e1b784271` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DESIGN.md](DESIGN.md) | `8a1b32338596cf713982f19490438ae337d73660425477be34e6477bc392cecf` | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | `46972609ec619a10d68b9a99613892f76fa8835129d7517bc484760d3dd309d0` | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | `023473c83d47138aa84c68abac7cec71409b33408781c6f513864d9c82f251bb` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/STANDARDS.md](STANDARDS.md) | `bbf98be2606978e6ec5e00add325d1d22fe899e8f3703f117733dc824998b2bb` | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/something.spec.md](something.spec.md) | `89d3ff3f68129830ea79e558f93e35e2ef461710c9cbee88b127f0c6563cd40f` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END GENERATED DOCUMENT STATE -->

## Commands

Use `./build/doccheck state check` or `make check-docs` to compare exact bytes without writing. After reviewing each changed document and the dependency guidance above, use `make docs-state-update` to acknowledge the reviewed state.
