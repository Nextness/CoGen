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
| [docs/APP-USAGE.md](APP-USAGE.md) | `b03acc6f8b6d93e095ed500490f5658ce8e84a62d7a686055b0c2261bf3e7800` | None |
| [docs/ARCHITECTURE.md](ARCHITECTURE.md) | `9a848db607d115a8ce3265cdf40dcf58cb9ab8d18026cc73828b07fe79f59965` | [docs/DATABASE.md](DATABASE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/APP-USAGE.md](APP-USAGE.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/CSS-REFERENCE.md](CSS-REFERENCE.md) | `ed3fd253112cfb8c1480fda3f89e45684560cd91baaed7dba9a1f7943dc16be8` | None |
| [docs/DATABASE.md](DATABASE.md) | `cb175a3b1273f59b157e3fc0fb9518f6e00055bb2a6c399e941b7a315b33a49a` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DESIGN.md](DESIGN.md) | `c94edaf41d1598c1671f62951680494160295126c2fc04fe6cf8de355eab30f8` | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) | `3edd85cb5774ae91f0f025a6f3ce225916d4e7ea53453078ed0402d58a88e3c2` | [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/JSX-RUNTIME.md](JSX-RUNTIME.md) | `4247fb288a172b75dc48479a3de2fb1c8983df068e06101424a71ca59524a699` | [docs/DESIGN.md](DESIGN.md), [docs/STANDARDS.md](STANDARDS.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | `250b406ab171f088b45b0afc71028a2a69da934c9941fb08ea162e6f80068337` | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | `88b521dca4abd855082409719e51ce865ca450a8ec82c2310d3d6843357f3901` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/STANDARDS.md](STANDARDS.md) | `ab51cf9c941b05ab43cfba5fda192ac7ab661de83e4f6e33d07310694ca32660` | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md), [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) |
| [docs/something.spec.md](something.spec.md) | `547af4593f9d0dd0c226ab0be0b29577e2e999d3cc369f4351517412d678d447` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END GENERATED DOCUMENT STATE -->

## Commands

Use `./build/doccheck state check` or `make check-docs` to compare exact bytes without writing. After reviewing each changed document and the dependency guidance above, use `make docs-state-update` to acknowledge the reviewed state.
