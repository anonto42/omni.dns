# Contributing to DNS Server

We welcome contributions! Please follow these guidelines to ensure a smooth collaboration process.

## Workflow

1.  **Fork and Branch:** Fork the repository, create a feature branch (`git checkout -b feat/your-feature`).
2.  **Implement:** Make your changes, following the architectural standards in `docs/`.
3.  **Verify:** Run `make lint` or `make test` locally to ensure your changes don't break existing functionality or introduce linting errors.
4.  **Commit:** Create descriptive commits. Every commit must be signed off using `git commit -s`.
5.  **Pull Request:** Open a Pull Request against the `main` branch.

## Standards

- **Commit Messages:** Summarize "why" the change was made.
- **Verification:** All commits must pass `make test`. The `make lint` target is kept as a contributor-friendly alias for the same repository checks.
- **Documentation:** If you add new functionality, update the relevant documentation in `docs/` following the guidelines in `docs/README.md`.

---
*For more information on the project architecture, please refer to the `docs/` directory.*
