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
| [docs/CODE-GRAPH-RAG-DISCUSSION.md](CODE-GRAPH-RAG-DISCUSSION.md) | [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/something.spec.md](something.spec.md) | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END DOCUMENT REVIEW DEPENDENCIES -->

## Acknowledged file state

<!-- BEGIN GENERATED DOCUMENT STATE -->

| Document | SHA-256 | Review dependents |
|---|---|---|
| [docs/APP-USAGE.md](APP-USAGE.md) | `f7896df8a1d9a478657e1db01ac3ae46e0516566430c4bfee4d10215cd4c0b3f` | None |
| [docs/ARCHITECTURE.md](ARCHITECTURE.md) | `10ffd7593b2d617b6192e617d98ff2cf7c6036ec2c4eb52e7bd21cd6f32acad1` | [docs/DATABASE.md](DATABASE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/APP-USAGE.md](APP-USAGE.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/CODE-GRAPH-RAG-DISCUSSION.md](CODE-GRAPH-RAG-DISCUSSION.md) | `d7e0d8b5b04a05d6a4ad84f598a05df62a32e176811b48c4ff2668d588cce911` | [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/CSS-REFERENCE.md](CSS-REFERENCE.md) | `7c81c957cf4ebe6f38d47c37bd1f06a4571457ac75b279160049ee0488689ae9` | None |
| [docs/DATABASE.md](DATABASE.md) | `ed56316fd9ca87df64c5526bd270a41f1cc1eb246c892cac70e50f0bcdd66642` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DESIGN.md](DESIGN.md) | `05460f5a37a2333367db9a28927f30338e7dc317e5d91033a7a9c22567e0b81d` | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) | `3edd85cb5774ae91f0f025a6f3ce225916d4e7ea53453078ed0402d58a88e3c2` | [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/JSX-RUNTIME.md](JSX-RUNTIME.md) | `4247fb288a172b75dc48479a3de2fb1c8983df068e06101424a71ca59524a699` | [docs/DESIGN.md](DESIGN.md), [docs/STANDARDS.md](STANDARDS.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | `0ed71163dfbf3f73c2fe9c780999adfb126cf51c41a6bd8471100be6a0ee9c19` | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | `35c6a9b8e02e17dde075fdf6fead26d42dd9f5ab28e83f88640ba0e13357a412` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/STANDARDS.md](STANDARDS.md) | `2980d03735b47a7472dea42b40d04f555274e21391c9b5d95dc716fc35fd4a39` | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md), [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) |
| [docs/something.spec.md](something.spec.md) | `547af4593f9d0dd0c226ab0be0b29577e2e999d3cc369f4351517412d678d447` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END GENERATED DOCUMENT STATE -->

## Commands

Use `./build/doccheck state check` or `make check-docs` to compare exact bytes without writing. After reviewing each changed document and the dependency guidance above, use `make docs-state-update` to acknowledge the reviewed state.
