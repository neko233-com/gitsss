# GitLab repository conventions

gitsss keeps the Gitea-compatible repository layout while accepting common
GitLab repository conventions during migration and normal operation.

- `.gitlab/CODEOWNERS` participates in automatic pull-request reviewer
  selection and protected-branch approval enforcement.
- `.gitlab/merge_request_templates/Default.md` is used as the default
  pull-request description template.
- `.gitlab/MERGE_REQUEST_TEMPLATE.md` is accepted for repositories that keep a
  single merge-request template.
- `.gitlab/issue_templates/*.md` is discovered in the issue-template chooser.
- `.gitlab/ISSUE_TEMPLATE.md` and lowercase equivalents are accepted as the
  default new-issue description.

Existing `CODEOWNERS`, `docs/CODEOWNERS`, `.gitea/`, and `.github/` conventions
continue to work. The first matching file in the documented candidate order is
used, so an existing repository does not silently change precedence.
