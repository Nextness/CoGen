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
| [docs/APP-USAGE.md](APP-USAGE.md) | `a17ea4b018ceac4589023be5af8a18e73b04bf6464fbed193948e1679d7bc33c` | None |
| [docs/ARCHITECTURE.md](ARCHITECTURE.md) | `dff99d2096c9f3696e953778e17f81698c52882ad7f65e0d73e3d848227267df` | [docs/DATABASE.md](DATABASE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/APP-USAGE.md](APP-USAGE.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/CSS-REFERENCE.md](CSS-REFERENCE.md) | `46f01ac32cb21e78c1d30b7f7b5d654787a0e2261e519fa75da277863da14712` | None |
| [docs/DATABASE.md](DATABASE.md) | `2c7a9b572ffb505f6a3c3bfeefbf95b124e809e61254cbd38b1d8894fae5ca64` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |
| [docs/DESIGN.md](DESIGN.md) | `7eb5805672f13bedda8c9f8b6f697bfe5316547cadf52978401a18cc2d697a2c` | [docs/CSS-REFERENCE.md](CSS-REFERENCE.md), [docs/APP-USAGE.md](APP-USAGE.md) |
| [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) | `fead9f9b4138651df40e76cf89504970d15217183ce087c956db56e20dbe91b0` | [docs/STANDARDS.md](STANDARDS.md), [docs/DESIGN.md](DESIGN.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/JSX-RUNTIME.md](JSX-RUNTIME.md) | `ac8105ffe819e9407196664b729d3aeb8d8350994154709598a4c6ada09b2bf2` | [docs/DESIGN.md](DESIGN.md), [docs/STANDARDS.md](STANDARDS.md), [docs/ARCHITECTURE.md](ARCHITECTURE.md) |
| [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) | `8d867de280f202fbd55ff1b006dc125c63c594d98884140e9f3411597bb6a92f` | None |
| [docs/PROJECT_CATALOG.md](PROJECT_CATALOG.md) | `19eab74caea30029fc533c320844853f7e7088b90a00c939d9bb389bc0c95183` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md) |
| [docs/STANDARDS.md](STANDARDS.md) | `72c8765ff677a3eb33688aea42e4cb7a7bb96e8f36d2a4b45ee3060d0e138ccd` | [docs/DESIGN.md](DESIGN.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md), [docs/FRONTEND-CODE-STYLE-GUIDE.md](FRONTEND-CODE-STYLE-GUIDE.md) |
| [docs/something.spec.md](something.spec.md) | `e8ec2bd767083db16af815e27c7b30e7a3c997b65adf38c028bd8de31b405d8d` | [docs/ARCHITECTURE.md](ARCHITECTURE.md), [docs/STANDARDS.md](STANDARDS.md), [docs/PROJECT-USAGE.md](PROJECT-USAGE.md) |

<!-- END GENERATED DOCUMENT STATE -->

## Commands

Use `./build/doccheck state check` or `make check-docs` to compare exact bytes without writing. After reviewing each changed document and the dependency guidance above, use `make docs-state-update` to acknowledge the reviewed state.
