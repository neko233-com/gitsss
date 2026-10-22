# Kanban layers

gitsss keeps the project board for three scopes, plus the organization quick-task
board.

- A repository board at `/{owner}/{repo}/projects` holds issues from that
  repository. Columns can be added, renamed, deleted, and given a WIP hint.
  Custom fields are a JSON list of text, number, user, and select fields.
  Cards drag between columns.
- A user board at `/{username}/-/projects` collects issues from repositories
  that user owns. The same columns, fields, and drag behavior apply. The board
  page accepts `owner/repo#123`.
- An organization board at `/{org}/-/projects` collects issues from
  repositories that organization owns, with the same controls.

Adding `owner/repo#123` keeps the issue on its repository board and also places
it on the user or organization board. The card shows `owner/repo` when the
board is not inside one repository.

`/{org}/-/kanban` remains the organization quick-task board for short cards
that are not Git issues. Its columns are To do, In progress, and Done.
