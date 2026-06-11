# Legacy data migration

Moves data from the **old** Tanabata database (the Python/Flask version, schema
in [`docs/reference/schema.sql`](../../docs/reference/schema.sql)) into the
**new** `core` / `data` / `acl` / `activity` schema.

- [`transform.sql`](transform.sql) — the actual data transformation. Reads a
  `legacy` schema (the old tables) and writes the new schema, in one
  transaction. Idempotent.
- [`migrate.sh`](migrate.sh) — links the new DB to the old one via
  `postgres_fdw`, imports the old `public` schema as `legacy`, runs
  `transform.sql`, then removes the link. The old DB is only **read**.

Tested end-to-end against PostgreSQL 16 (schema applied, synthetic legacy data,
all transformations + idempotency verified).

## Prerequisites

1. The **new** schema exists and is seeded — start the app once (it runs the
   goose migrations incl. `007_seed_data`), or run goose manually.
2. `NEW_DSN` connects as a role allowed to `CREATE EXTENSION postgres_fdw`
   (a superuser — the compose Postgres' `POSTGRES_USER` is one).
3. The new Postgres server can reach the old DB host over the network.
4. `psql` on PATH.

## Run

```bash
cd scripts/migrate-legacy

NEW_DSN='postgres://tanabata:PASS@localhost:42777/tanabata' \
OLD_HOST=192.168.1.10 OLD_PORT=5432 OLD_DB=tfm \
OLD_USER=hiko OLD_PASSWORD=SECRET \
./migrate.sh
```

It prints the source (legacy) row counts, then the resulting new-schema counts.
Re-running is safe — `ON CONFLICT DO NOTHING` everywhere means a second run only
fills in what is missing.

### Without postgres_fdw

`transform.sql` only needs the old tables to be visible as a `legacy` schema. If
you'd rather not use fdw, load the old dump into a schema named `legacy` in the
new database by whatever means, then run just the transform:

```bash
psql "$NEW_DSN" -v ON_ERROR_STOP=1 -f transform.sql
```

## What gets migrated, and how

| Old (`public`)        | New                     | Notes |
|-----------------------|-------------------------|-------|
| `users`               | `core.users`            | id **uuid → smallint** (remapped by unique `name`); `can_edit` → `can_create`; `is_blocked` = false |
| `mime`                | `core.mime_types`       | id **uuid → smallint** (remapped by `name`); types not already seeded are added |
| `categories`          | `data.categories`       | id kept; `is_private` → **`is_public`** (inverted) |
| `tags`                | `data.tags`             | id + `category_id` kept; inverted privacy |
| `autotags`            | `data.tag_rules`        | `child_id` → `when_tag_id`, `parent_id` → `then_tag_id` (old: adding the child pulled in the parent) |
| `files`               | `data.files`            | id kept; `datetime` → `content_datetime`; `orig_name` → `original_name`; **EXIF** lifted from `metadata->'exif'` into the `exif` column, the rest stays as user `metadata` |
| `file_tag`            | `data.file_tag`         | orphan rows skipped |
| `pools`               | `data.pools`            | id kept; `parent_id` + `created` preserved under `metadata` (see below) |
| `file_pool`           | `data.file_pool`        | `position` synthesised (gapped 1000s, ordered by file id) |
| `acl`                 | `acl.permissions`       | object type **derived** by locating the object; `read`/`write` → `can_view`/`can_edit` |
| `file_views`          | `activity.file_views`   | `datetime` → `viewed_at` |

Throughout: empty `notes` (`''`) → `NULL`; colours that aren't 6-hex are set to
`NULL` (the old `CHECK` was `NOT VALID`, so bad values could exist).

### Decisions / lossy points

- **Passwords** are copied verbatim. If the old hashes are bcrypt (as the new
  app expects) logins keep working; otherwise affected users need a reset.
- **`created` timestamps** on categories/tags/files are dropped — their UUIDv7
  ids already encode creation time. Pools use random v4 ids, so their `created`
  (and the dropped **pool hierarchy** `parent_id`) are preserved under
  `data.pools.metadata` as `legacy_created` / `legacy_parent_id`.
- **`file_pool` ordering**: the old schema stored none, so position is generated
  from file-id order (≈ chronological) with gaps of 1000.
- **Not migrated**: `sessions` / `user_agents` — the new app uses JWTs, so users
  simply log in again. There were no audit-log / pool-view / tag-use tables in
  the old schema, so those start empty. `phash` and `is_deleted` are new
  (`NULL` / `false`).

## Physical files (separate, manual)

The script migrates the **database only**. File blobs must be copied too. The
new layout stores originals at `FILES_PATH/{uuid}` with **no extension**;
thumbnails/previews are regenerated on demand, so don't copy those. Because ids
are preserved, the old `{uuid}.{ext}` files map 1:1 — just strip the extension:

```bash
OLD_FILES=/srv/old-tanabata/files     # old originals ({uuid}.{ext})
NEW_FILES=/var/lib/tanabata/files     # new FILES_PATH

for src in "$OLD_FILES"/*; do
  id="$(basename "$src")"; id="${id%.*}"   # uuids contain no dots
  cp -n "$src" "$NEW_FILES/$id"
done

# Make them readable by the container user (uid/gid 42776):
chown -R 42776:42776 "$NEW_FILES"
```
