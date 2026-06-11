#!/usr/bin/env bash
# =============================================================================
# Tanabata legacy -> new schema migration (orchestrator)
#
# Connects the NEW database to the OLD one via postgres_fdw, imports the old
# `public` schema as `legacy`, runs transform.sql (the actual data move, in one
# transaction), then tears the foreign link down again. The OLD database is
# only read.
#
# Prerequisites:
#   - The NEW schema already exists and is seeded (start the app once, or run
#     goose, so all migrations incl. 007_seed_data have applied).
#   - NEW_DSN connects as a role allowed to CREATE EXTENSION postgres_fdw
#     (a superuser; the compose Postgres' POSTGRES_USER is one).
#   - The NEW Postgres server can reach OLD_HOST:OLD_PORT over the network.
#   - `psql` is on PATH.
#
# Usage:
#   NEW_DSN='postgres://tanabata:pass@localhost:42777/tanabata' \
#   OLD_HOST=192.168.1.10 OLD_DB=tfm OLD_USER=hiko OLD_PASSWORD=secret \
#   ./migrate.sh
# =============================================================================
set -euo pipefail

# --- Config from the environment --------------------------------------------
NEW_DSN="${NEW_DSN:?set NEW_DSN to the new database connection string}"
OLD_HOST="${OLD_HOST:?set OLD_HOST}"
OLD_PORT="${OLD_PORT:-5432}"
OLD_DB="${OLD_DB:?set OLD_DB (old database name)}"
OLD_USER="${OLD_USER:?set OLD_USER}"
OLD_PASSWORD="${OLD_PASSWORD:?set OLD_PASSWORD}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TRANSFORM_SQL="$SCRIPT_DIR/transform.sql"

psql_new() { psql "$NEW_DSN" -v ON_ERROR_STOP=1 "$@"; }

# --- Always remove the foreign link on exit, success or failure -------------
teardown() {
    psql "$NEW_DSN" -q >/dev/null 2>&1 <<'SQL' || true
DROP SCHEMA IF EXISTS legacy CASCADE;
DROP SERVER IF EXISTS legacy_src CASCADE;
SQL
}
trap teardown EXIT

echo ">> Linking NEW database to OLD ($OLD_USER@$OLD_HOST:$OLD_PORT/$OLD_DB) via postgres_fdw ..."
psql_new \
    -v old_host="$OLD_HOST" \
    -v old_port="$OLD_PORT" \
    -v old_db="$OLD_DB" \
    -v old_user="$OLD_USER" \
    -v old_pw="$OLD_PASSWORD" <<'SQL'
CREATE EXTENSION IF NOT EXISTS postgres_fdw;

-- Start clean in case a previous run was interrupted.
DROP SCHEMA IF EXISTS legacy CASCADE;
DROP SERVER IF EXISTS legacy_src CASCADE;

CREATE SERVER legacy_src FOREIGN DATA WRAPPER postgres_fdw
    OPTIONS (host :'old_host', port :'old_port', dbname :'old_db');

-- :'old_user' / :'old_pw' are quoted+escaped by psql, so passwords with
-- special characters are safe.
CREATE USER MAPPING FOR CURRENT_USER SERVER legacy_src
    OPTIONS (user :'old_user', password :'old_pw');

CREATE SCHEMA legacy;
IMPORT FOREIGN SCHEMA public LIMIT TO (
    users, mime, categories, tags, autotags, files, file_tag, pools, file_pool, acl, file_views
) FROM SERVER legacy_src INTO legacy;
SQL

echo ">> Source (legacy) row counts:"
psql_new -P pager=off -c "
SELECT 'users' AS table, count(*) FROM legacy.users
UNION ALL SELECT 'mime',       count(*) FROM legacy.mime
UNION ALL SELECT 'categories', count(*) FROM legacy.categories
UNION ALL SELECT 'tags',       count(*) FROM legacy.tags
UNION ALL SELECT 'autotags',   count(*) FROM legacy.autotags
UNION ALL SELECT 'files',      count(*) FROM legacy.files
UNION ALL SELECT 'file_tag',   count(*) FROM legacy.file_tag
UNION ALL SELECT 'pools',      count(*) FROM legacy.pools
UNION ALL SELECT 'file_pool',  count(*) FROM legacy.file_pool
UNION ALL SELECT 'acl',        count(*) FROM legacy.acl
UNION ALL SELECT 'file_views', count(*) FROM legacy.file_views
ORDER BY 1;"

echo ">> Running transform (single transaction) ..."
psql_new -P pager=off -f "$TRANSFORM_SQL"

echo ">> Done. The foreign link will be removed now."
