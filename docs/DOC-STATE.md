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
| [docs/APP-USAGE.md](APP-USAGE.md) | `16a1cf6f117cfd7cda303f575fd0ab038309e78075214bdb06f3474fe2d71590` | None |
| [docs/ARCHITECTURE.md](ARCHITECTURE.md) | `482c33503e3a533399e7939d88630505bdb9c70209f8ad35b1afbd675f187098` | [docs/DATABASE.md](DATABASE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/APP-USAGE.md](APP-USAGE.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/CSS-REFERENCE.md](CSS-REFERENCE.md) | `46f01ac32cb21e78c1d30b7f7b5d654787a0e2261e519fa75da277863da14712` | None |
| [docs/DATABASE.md](DATABASE.md) | `b41d377f8c4444b43cd3e95b242339b4d935e9eef6cf76e3fa08ca7cc547389b` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DESIGN.md](DESIGN.md) | `861901dca832db651392e44cfdebb6e9d974bf77c0693c4b30ea2575ae0ba45f` | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) | `fead9f9b4138651df40e76cf89504970d15217183ce087c956db56e20dbe91b0` | [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/JSX-RUNTIME.md](JSX-RUNTIME.md) | `e9d717fe99e9494a3c7033aea5a52d04778bc81ec7278b70de3542590f341def` | [docs/DESIGN.md](DESIGN.md), [docs/STANDARDS.md](STANDARDS.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | `9d25a9ecbc95f29e75e89b20a2c73d35a579384dc600983b04fc36cd4e84aae1` | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | `48daeada7d42862ece0596273adaa3f50a2e2944da4f478421f7ca939c70646a` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/STANDARDS.md](STANDARDS.md) | `612b661e4d8305da19e9d92500a418ab0ebd60c9cc275f774340beef901c6c08` | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md), [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) |
| [docs/something.spec.md](something.spec.md) | `89d3ff3f68129830ea79e558f93e35e2ef461710c9cbee88b127f0c6563cd40f` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END GENERATED DOCUMENT STATE -->

## Commands

Use `./build/doccheck state check` or `make check-docs` to compare exact bytes without writing. After reviewing each changed document and the dependency guidance above, use `make docs-state-update` to acknowledge the reviewed state.
