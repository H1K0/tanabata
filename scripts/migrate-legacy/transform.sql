-- =============================================================================
-- Tanabata legacy -> new schema data migration (transform step)
--
-- Reads the OLD database (exposed as the `legacy` schema — see migrate.sh, which
-- imports it via postgres_fdw) and inserts the transformed rows into the new
-- core / data / acl / activity schemas.
--
-- Assumes the new schema already exists (goose migrations applied) and is seeded
-- (core.mime_types, core.object_types from 007_seed_data.sql).
--
-- Idempotent: ON CONFLICT DO NOTHING everywhere + preserved UUID PKs, so a
-- re-run inserts only what is missing. Runs as one transaction — all or nothing.
--
-- Run with:  psql "<new-dsn>" -v ON_ERROR_STOP=1 -f transform.sql
-- (migrate.sh does this for you after setting up the `legacy` schema.)
-- =============================================================================

\set ON_ERROR_STOP on

-- Fail early and clearly if the legacy data hasn't been made available.
DO $$
BEGIN
    IF to_regclass('legacy.users') IS NULL THEN
        RAISE EXCEPTION
            'legacy.* tables not found. Populate the "legacy" schema first '
            '(run migrate.sh, or load the old dump into a schema named legacy).';
    END IF;
END $$;

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Users.  Old PK is uuid; the new table uses a smallint identity. Insert by
--    the unique `name`, then build a uuid -> smallint map used by every FK below.
--    Old `can_edit` becomes the new `can_create`; nobody is blocked on import.
-- ---------------------------------------------------------------------------
INSERT INTO core.users (name, password, is_admin, can_create, is_blocked)
SELECT name, password, is_admin, can_edit, false
FROM legacy.users
ON CONFLICT (name) DO NOTHING;

CREATE TEMP TABLE user_id_map ON COMMIT DROP AS
SELECT lu.id AS old_id, nu.id AS new_id
FROM legacy.users lu
JOIN core.users nu ON nu.name = lu.name;

-- ---------------------------------------------------------------------------
-- 2. MIME types.  Same uuid -> smallint remap, keyed by the MIME name. The new
--    DB is pre-seeded with the common types; add any legacy ones not seeded.
-- ---------------------------------------------------------------------------
INSERT INTO core.mime_types (name, extension)
SELECT name, extension
FROM legacy.mime
ON CONFLICT (name) DO NOTHING;

CREATE TEMP TABLE mime_id_map ON COMMIT DROP AS
SELECT lm.id AS old_id, nm.id AS new_id
FROM legacy.mime lm
JOIN core.mime_types nm ON nm.name = lm.name;

-- ---------------------------------------------------------------------------
-- 3. Categories.  UUID PK preserved. is_private -> is_public (inverted),
--    '' notes -> NULL, non-hex colors -> NULL (to satisfy the hex CHECK that the
--    old NOT VALID constraint may not have enforced on existing rows).
-- ---------------------------------------------------------------------------
INSERT INTO data.categories (id, name, notes, color, metadata, creator_id, is_public)
SELECT c.id,
       c.name,
       NULLIF(c.notes, ''),
       CASE WHEN c.color ~* '^[A-Fa-f0-9]{6}$' THEN c.color END,
       NULL,
       um.new_id,
       NOT c.is_private
FROM legacy.categories c
JOIN user_id_map um ON um.old_id = c.creator_id
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 4. Tags.  UUID PK + category_id preserved.
-- ---------------------------------------------------------------------------
INSERT INTO data.tags (id, name, notes, color, category_id, metadata, creator_id, is_public)
SELECT t.id,
       t.name,
       NULLIF(t.notes, ''),
       CASE WHEN t.color ~* '^[A-Fa-f0-9]{6}$' THEN t.color END,
       t.category_id,
       NULL,
       um.new_id,
       NOT t.is_private
FROM legacy.tags t
JOIN user_id_map um ON um.old_id = t.creator_id
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 5. Tag rules (old `autotags`).  In the old schema adding the child tag pulled
--    in its parent (tfm__add_file_to_tag_recursive: for child_id = t_id it adds
--    parent_id). The new model is "when_tag applied -> then_tag follows", so the
--    trigger child becomes when_tag and the auto-applied parent becomes then_tag.
--    Skip rules whose tags didn't migrate.
-- ---------------------------------------------------------------------------
INSERT INTO data.tag_rules (when_tag_id, then_tag_id, is_active)
SELECT a.child_id, a.parent_id, a.is_active
FROM legacy.autotags a
WHERE EXISTS (SELECT 1 FROM data.tags t WHERE t.id = a.child_id)
  AND EXISTS (SELECT 1 FROM data.tags t WHERE t.id = a.parent_id)
ON CONFLICT (when_tag_id, then_tag_id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 6. Files.  UUID PK preserved. old `datetime` -> content_datetime,
--    `orig_name` -> original_name. EXIF is lifted out of the old metadata blob
--    into its own column; whatever else was in metadata stays as user metadata
--    (NULL if nothing remains). No phash / soft-delete existed before.
-- ---------------------------------------------------------------------------
INSERT INTO data.files (id, original_name, mime_id, content_datetime, notes,
                        metadata, exif, phash, creator_id, is_public, is_deleted)
SELECT f.id,
       f.orig_name,
       mm.new_id,
       f.datetime,
       NULLIF(f.notes, ''),
       NULLIF(f.metadata - 'exif', '{}'::jsonb),
       f.metadata -> 'exif',
       NULL,
       um.new_id,
       NOT f.is_private,
       false
FROM legacy.files f
JOIN user_id_map um ON um.old_id = f.creator_id
JOIN mime_id_map mm ON mm.old_id = f.mime_id
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 7. File <-> tag.  Skip orphan junction rows.
-- ---------------------------------------------------------------------------
INSERT INTO data.file_tag (file_id, tag_id)
SELECT ft.file_id, ft.tag_id
FROM legacy.file_tag ft
WHERE EXISTS (SELECT 1 FROM data.files f WHERE f.id = ft.file_id)
  AND EXISTS (SELECT 1 FROM data.tags  t WHERE t.id = ft.tag_id)
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- 8. Pools.  UUID PK preserved. The new schema has neither pool hierarchy nor a
--    `created` column, so the legacy parent_id and created timestamp are kept
--    under metadata (pool ids are random v4, so created isn't otherwise
--    recoverable). is_private -> is_public.
-- ---------------------------------------------------------------------------
INSERT INTO data.pools (id, name, notes, metadata, creator_id, is_public)
SELECT p.id,
       p.name,
       NULLIF(p.notes, ''),
       jsonb_strip_nulls(jsonb_build_object(
           'legacy_parent_id', p.parent_id,
           'legacy_created',   p.created)),
       um.new_id,
       NOT p.is_private
FROM legacy.pools p
JOIN user_id_map um ON um.old_id = p.creator_id
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 9. File <-> pool.  The old table has no ordering column; synthesise a stable
--    gapped position per pool, ordered by file id (UUID v7 ≈ chronological), so
--    the app's gap-based reordering keeps working.
-- ---------------------------------------------------------------------------
INSERT INTO data.file_pool (file_id, pool_id, position)
SELECT fp.file_id,
       fp.pool_id,
       (row_number() OVER (PARTITION BY fp.pool_id ORDER BY fp.file_id))::int * 1000
FROM legacy.file_pool fp
WHERE EXISTS (SELECT 1 FROM data.files f WHERE f.id = fp.file_id)
  AND EXISTS (SELECT 1 FROM data.pools p WHERE p.id = fp.pool_id)
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- 10. ACL.  The old table stored no object type; derive it by locating the
--     object among files/tags/categories/pools. read/write -> can_view/can_edit.
--     Rows whose object no longer exists are skipped.
-- ---------------------------------------------------------------------------
INSERT INTO acl.permissions (user_id, object_type_id, object_id, can_view, can_edit)
SELECT um.new_id, ot.id, a.object_id, a.read, a.write
FROM legacy.acl a
JOIN user_id_map um ON um.old_id = a.user_id
JOIN LATERAL (
    SELECT CASE
        WHEN EXISTS (SELECT 1 FROM data.files      f WHERE f.id = a.object_id) THEN 'file'
        WHEN EXISTS (SELECT 1 FROM data.tags       t WHERE t.id = a.object_id) THEN 'tag'
        WHEN EXISTS (SELECT 1 FROM data.categories c WHERE c.id = a.object_id) THEN 'category'
        WHEN EXISTS (SELECT 1 FROM data.pools      p WHERE p.id = a.object_id) THEN 'pool'
    END AS type_name
) k ON true
JOIN core.object_types ot ON ot.name = k.type_name
ON CONFLICT (user_id, object_type_id, object_id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 11. File view history.  old `datetime` -> viewed_at.
-- ---------------------------------------------------------------------------
INSERT INTO activity.file_views (file_id, user_id, viewed_at)
SELECT fv.file_id, um.new_id, fv.datetime
FROM legacy.file_views fv
JOIN user_id_map um ON um.old_id = fv.user_id
WHERE EXISTS (SELECT 1 FROM data.files f WHERE f.id = fv.file_id)
ON CONFLICT DO NOTHING;

COMMIT;

-- ---------------------------------------------------------------------------
-- Summary of what now lives in the new schema.
-- ---------------------------------------------------------------------------
\echo ''
\echo 'Migration committed. New row counts:'
SELECT 'core.users'            AS table, count(*) FROM core.users
UNION ALL SELECT 'core.mime_types',        count(*) FROM core.mime_types
UNION ALL SELECT 'data.categories',        count(*) FROM data.categories
UNION ALL SELECT 'data.tags',              count(*) FROM data.tags
UNION ALL SELECT 'data.tag_rules',         count(*) FROM data.tag_rules
UNION ALL SELECT 'data.files',             count(*) FROM data.files
UNION ALL SELECT 'data.file_tag',          count(*) FROM data.file_tag
UNION ALL SELECT 'data.pools',             count(*) FROM data.pools
UNION ALL SELECT 'data.file_pool',         count(*) FROM data.file_pool
UNION ALL SELECT 'acl.permissions',        count(*) FROM acl.permissions
UNION ALL SELECT 'activity.file_views',    count(*) FROM activity.file_views
ORDER BY 1;
