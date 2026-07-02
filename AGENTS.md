# AGENTS.md

## Changelog Guidelines

When cutting a release, update `src/content/docs/changelog.md` before tagging.

- Add a new section for the exact version tag being released.
- Keep the newest version at the top.
- Skip versions that do not have git tags.
- Use commit history and diffs on `main` to summarize code changes.
- This is an OSS project, so internal code changes may be included when useful.
- Do not include docs-site-only changes such as site styling, Zuedocs/package bumps, deploy plumbing, footer/layout changes, or documentation navigation changes.
- Rewrite commit subjects into clear release notes instead of pasting raw commit messages.
- If a release contains only tagging/release metadata, write: `Maintenance release. No direct code behavior changes beyond release preparation.`
