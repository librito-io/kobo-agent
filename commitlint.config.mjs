// Commit message rules — enforced in CI via .github/workflows/commitlint.yml.
// This is a Go repo with no Node runtime, so there is NO local husky hook
// (unlike librito-io/web); the CI backstop is the only enforcement of commit
// message shape. PR titles are validated separately by
// .github/workflows/lint-pr-title.yml (amannn/action-semantic-pull-request).
// Full convention lives inline in CLAUDE.md → "PR & Commit Convention".

/** @type {import('@commitlint/types').UserConfig} */
export default {
  extends: ["@commitlint/config-conventional"],
  rules: {
    // Adds `bug` to the conventional-commits default 8 types — Librito
    // convention for "user-facing defect fix" distinct from `fix` for
    // internal-only regressions.
    "type-enum": [
      2,
      "always",
      ["feat", "fix", "bug", "chore", "docs", "test", "perf", "refactor"],
    ],
    // 100 catches genuine runaway subjects without rejecting substantive
    // conjunctive subjects ("X + Y", "replace X with Y"). Soft targets of
    // 50 / 72 live in CLAUDE.md as guidance, not as a gate.
    "header-max-length": [2, "always", 100],
    // Disable per-line body/footer wrap — modern consumers soft-wrap; no
    // git-format-patch / mailing-list workflow needs fixed width.
    "body-max-line-length": [0],
    "footer-max-line-length": [0],
  },
};
