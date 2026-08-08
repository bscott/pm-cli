# Batch Operation Format

`pm-cli mail batch` reads a JSON array of operations from stdin (or from a file via `--file`) and executes them in order over a single IMAP connection.

## Input

A JSON array of operation objects:

```json
[
  {"op": "label",   "uids": ["uid:123"], "label": "Important"},
  {"op": "flag",    "uids": ["uid:456"], "read": true},
  {"op": "archive", "uids": ["uid:789"]}
]
```

Input is limited to 10MB. An empty array, malformed JSON, or any invalid operation causes the whole batch to be rejected **before** a connection is opened — nothing is executed.

## Operation fields

| Field | Type | Applies to | Description |
|-------|------|-----------|-------------|
| `op` | string | all | One of `label`, `unlabel`, `archive`, `move`, `flag`, `delete`. **Required.** |
| `uids` | string[] | all | Message selectors — `uid:<n>` or a bare sequence number. **Required, non-empty.** Do not mix UID and sequence selectors within one operation. |
| `mailbox` | string | all | Source mailbox. Defaults to `INBOX`. |
| `label` | string | `label`, `unlabel` | Label name (mapped to the `Labels/<name>` folder). **Required** for these ops. |
| `to` | string | `move` | Destination mailbox. **Required** for `move`. |
| `read` | bool | `flag` | Mark messages read (`\Seen`). |
| `unread` | bool | `flag` | Mark messages unread (remove `\Seen`). |
| `star` | bool | `flag` | Star messages (`\Flagged`). |
| `unstar` | bool | `flag` | Unstar messages (remove `\Flagged`). |

A `flag` operation requires at least one of `read`, `unread`, `star`, `unstar`.

Mailbox and label names containing IMAP special characters (`{`, `*`, `%`, CR, LF) are rejected.

## Operations

| op | Effect |
|----|--------|
| `label` | Copies the messages into `Labels/<label>` (adds the label; original is kept). |
| `unlabel` | Removes the messages from `Labels/<label>` (removes the label). |
| `archive` | Moves the messages from `mailbox` to `Archive`. |
| `move` | Moves the messages from `mailbox` to `to`. |
| `flag` | Adds/removes `\Seen` / `\Flagged` per the boolean fields. |
| `delete` | Marks the messages deleted in `mailbox` (moves to Trash; not a permanent expunge). |

## Output

With `--json`, the command prints per-operation results plus totals:

```json
{
  "results": [
    {"op": "label", "success": true},
    {"op": "flag", "success": false, "error": "no messages matched the given ID(s) in INBOX"}
  ],
  "total": 2,
  "succeeded": 1,
  "failed": 1
}
```

By default every operation is attempted regardless of earlier failures. Pass `--stop-on-error` to halt after the first failure (the already-attempted results are still reported).

## Notes

- Operations execute sequentially in array order over one connection.
- There is no transaction/rollback: a partial batch leaves partial state. Inspect `results` to see exactly what succeeded.
- Accurate per-operation success reporting depends on the IMAP layer detecting no-op STORE/COPY commands. See issue #11.
