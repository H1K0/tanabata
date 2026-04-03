-- =============================================================================
-- Tanabata File Manager — Database Schema v2
-- =============================================================================
-- PostgreSQL 14+
--
-- Design decisions:
--   • Business logic lives in Go (DDD), no stored procedures
--   • UUID v7 for entity PKs (created_at extracted from UUID, no separate column)
--   • ACL: is_public flag on objects + acl.permissions table for granular control
--   • Schemas: core, data, acl, activity
--   • Flat pools (no hierarchy)
--   • Soft delete for files only (trash/recycle bin)
--   • phash field for future duplicate detection
--   • metadata jsonb on all entities
--   • Unified audit log with reference tables instead of enums
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Extensions
-- ---------------------------------------------------------------------------

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ---------------------------------------------------------------------------
-- Schemas
-- ---------------------------------------------------------------------------

CREATE SCHEMA IF NOT EXISTS core;
CREATE SCHEMA IF NOT EXISTS data;
CREATE SCHEMA IF NOT EXISTS acl;
CREATE SCHEMA IF NOT EXISTS activity;

-- ---------------------------------------------------------------------------
-- Utility functions
-- ---------------------------------------------------------------------------

-- UUID v7 generator
CREATE OR REPLACE FUNCTION public.uuid_v7(cts timestamptz DEFAULT clock_timestamp())
RETURNS uuid LANGUAGE plpgsql AS $$
DECLARE
    state text = current_setting('uuidv7.old_tp', true);
    old_tp text = split_part(state, ':', 1);
    base int = coalesce(nullif(split_part(state, ':', 4), '')::int, (random()*16777215/2-1)::int);
    tp text;
    entropy text;
    seq text = base;
    seqn int = split_part(state, ':', 2);
    ver text = coalesce(split_part(state, ':', 3), to_hex(8+(random()*3)::int));
BEGIN
    base = (random()*16777215/2-1)::int;
    tp = lpad(to_hex(floor(extract(epoch from cts)*1000)::int8), 12, '0') || '7';
    IF tp IS DISTINCT FROM old_tp THEN
        old_tp = tp;
        ver = to_hex(8+(random()*3)::int);
        base = (random()*16777215/2-1)::int;
        seqn = base;
    ELSE
        seqn = seqn + (random()*1000)::int;
    END IF;
    PERFORM set_config('uuidv7.old_tp', old_tp||':'||seqn||':'||ver||':'||base, false);
    entropy = md5(gen_random_uuid()::text);
    seq = lpad(to_hex(seqn), 6, '0');
    RETURN (tp || substring(seq from 1 for 3) || ver || substring(seq from 4 for 3) ||
            substring(entropy from 1 for 12))::uuid;
END;
$$;

-- Extract timestamp from UUID v7
CREATE OR REPLACE FUNCTION public.uuid_extract_timestamp(uuid_val uuid)
RETURNS timestamptz LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT to_timestamp(
        ('x' || left(replace(uuid_val::text, '-', ''), 12))::bit(48)::bigint / 1000.0
    );
$$;

-- =============================================================================
-- SCHEMA: core
-- =============================================================================

-- Users
CREATE TABLE core.users (
    id         smallint    GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       varchar(32) NOT NULL,
    password   text        NOT NULL,  -- bcrypt hash via pgcrypto
    is_admin   boolean     NOT NULL DEFAULT false,
    can_create boolean     NOT NULL DEFAULT false,

    CONSTRAINT uni__users__name UNIQUE (name)
);

-- MIME types (whitelist of supported file types)
CREATE TABLE core.mime_types (
    id        smallint     GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name      varchar(127) NOT NULL,
    extension varchar(16)  NOT NULL,

    CONSTRAINT uni__mime_types__name UNIQUE (name)
);

-- Object types (file, tag, category, pool — used in ACL and audit log)
CREATE TABLE core.object_types (
    id   smallint    GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name varchar(32) NOT NULL,

    CONSTRAINT uni__object_types__name UNIQUE (name)
);

-- =============================================================================
-- SCHEMA: data
-- =============================================================================

-- Categories (logical grouping of tags)
CREATE TABLE data.categories (
    id         uuid         NOT NULL DEFAULT public.uuid_v7() PRIMARY KEY,
    name       varchar(256) NOT NULL,
    notes      text,
    color      char(6),
    metadata   jsonb,
    creator_id smallint     NOT NULL REFERENCES core.users(id)
                            ON UPDATE CASCADE ON DELETE RESTRICT,
    is_public  boolean      NOT NULL DEFAULT false,

    CONSTRAINT uni__categories__name UNIQUE (name),
    CONSTRAINT chk__categories__color_hex
        CHECK (color IS NULL OR color ~* '^[A-Fa-f0-9]{6}$')
);

-- Tags
CREATE TABLE data.tags (
    id          uuid         NOT NULL DEFAULT public.uuid_v7() PRIMARY KEY,
    name        varchar(256) NOT NULL,
    notes       text,
    color       char(6),
    category_id uuid         REFERENCES data.categories(id)
                             ON UPDATE CASCADE ON DELETE SET NULL,
    metadata    jsonb,
    creator_id  smallint     NOT NULL REFERENCES core.users(id)
                             ON UPDATE CASCADE ON DELETE RESTRICT,
    is_public   boolean      NOT NULL DEFAULT false,

    CONSTRAINT uni__tags__name UNIQUE (name),
    CONSTRAINT chk__tags__color_hex
        CHECK (color IS NULL OR color ~* '^[A-Fa-f0-9]{6}$')
);

-- Tag rules (when when_tag is added to a file, then_tag is also added)
CREATE TABLE data.tag_rules (
    when_tag_id uuid    NOT NULL REFERENCES data.tags(id)
                        ON UPDATE CASCADE ON DELETE CASCADE,
    then_tag_id uuid    NOT NULL REFERENCES data.tags(id)
                        ON UPDATE CASCADE ON DELETE CASCADE,
    is_active   boolean NOT NULL DEFAULT true,

    PRIMARY KEY (when_tag_id, then_tag_id)
);

-- Files
CREATE TABLE data.files (
    id               uuid         NOT NULL DEFAULT public.uuid_v7() PRIMARY KEY,
    original_name    varchar(256),          -- original filename at upload time
    mime_id          smallint     NOT NULL REFERENCES core.mime_types(id)
                                  ON UPDATE CASCADE ON DELETE RESTRICT,
    content_datetime timestamptz  NOT NULL DEFAULT clock_timestamp(),  -- content datetime (e.g. photo taken)
    notes            text,
    metadata         jsonb,                 -- user-editable key-value data
    exif             jsonb        NOT NULL,  -- EXIF data extracted at upload (immutable)
    phash            bigint,                -- perceptual hash for duplicate detection (future)
    creator_id       smallint     NOT NULL REFERENCES core.users(id)
                                  ON UPDATE CASCADE ON DELETE RESTRICT,
    is_public        boolean      NOT NULL DEFAULT false,
    is_deleted       boolean      NOT NULL DEFAULT false  -- soft delete (trash)
);

-- File ↔ Tag (many-to-many)
CREATE TABLE data.file_tag (
    file_id uuid NOT NULL REFERENCES data.files(id)
                 ON UPDATE CASCADE ON DELETE CASCADE,
    tag_id  uuid NOT NULL REFERENCES data.tags(id)
                 ON UPDATE CASCADE ON DELETE CASCADE,

    PRIMARY KEY (file_id, tag_id)
);

-- Pools (ordered collections of files)
CREATE TABLE data.pools (
    id         uuid         NOT NULL DEFAULT public.uuid_v7() PRIMARY KEY,
    name       varchar(256) NOT NULL,
    notes      text,
    metadata   jsonb,
    creator_id smallint     NOT NULL REFERENCES core.users(id)
                            ON UPDATE CASCADE ON DELETE RESTRICT,
    is_public  boolean      NOT NULL DEFAULT false,

    CONSTRAINT uni__pools__name UNIQUE (name)
);

-- File ↔ Pool (many-to-many, with ordering)
-- `position` uses integer with gaps (e.g. 1000, 2000, 3000) to allow
-- insertions without renumbering. Compact when gaps get too small.
CREATE TABLE data.file_pool (
    file_id  uuid    NOT NULL REFERENCES data.files(id)
                     ON UPDATE CASCADE ON DELETE CASCADE,
    pool_id  uuid    NOT NULL REFERENCES data.pools(id)
                     ON UPDATE CASCADE ON DELETE CASCADE,
    position integer NOT NULL DEFAULT 0,

    PRIMARY KEY (file_id, pool_id)
);

-- =============================================================================
-- SCHEMA: acl
-- =============================================================================

-- Granular permissions
-- If is_public=true on the object, it is accessible to everyone (ACL ignored).
-- If is_public=false, only creator and users with can_view=true see it.
-- Admins bypass all ACL checks.
CREATE TABLE acl.permissions (
    user_id        smallint NOT NULL REFERENCES core.users(id)
                            ON UPDATE CASCADE ON DELETE CASCADE,
    object_type_id smallint NOT NULL REFERENCES core.object_types(id)
                            ON UPDATE CASCADE ON DELETE RESTRICT,
    object_id      uuid     NOT NULL,
    can_view       boolean  NOT NULL DEFAULT true,
    can_edit       boolean  NOT NULL DEFAULT false,

    PRIMARY KEY (user_id, object_type_id, object_id)
);

-- =============================================================================
-- SCHEMA: activity
-- =============================================================================

-- Action types (reference table for audit log)
CREATE TABLE activity.action_types (
    id   smallint    GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name varchar(64) NOT NULL,

    CONSTRAINT uni__action_types__name UNIQUE (name)
);

-- Sessions
CREATE TABLE activity.sessions (
    id            integer      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash    text         NOT NULL,  -- hashed session token
    user_id       smallint     NOT NULL REFERENCES core.users(id)
                               ON UPDATE CASCADE ON DELETE CASCADE,
    user_agent    varchar(256) NOT NULL,
    started_at    timestamptz  NOT NULL DEFAULT statement_timestamp(),
    expires_at    timestamptz,
    last_activity timestamptz  NOT NULL DEFAULT statement_timestamp(),

    CONSTRAINT uni__sessions__token_hash UNIQUE (token_hash)
);

-- File views (analytics)
CREATE TABLE activity.file_views (
    file_id    uuid        NOT NULL REFERENCES data.files(id)
                           ON UPDATE CASCADE ON DELETE CASCADE,
    user_id    smallint    NOT NULL REFERENCES core.users(id)
                           ON UPDATE CASCADE ON DELETE CASCADE,
    viewed_at  timestamptz NOT NULL DEFAULT statement_timestamp(),

    PRIMARY KEY (file_id, viewed_at, user_id)
);

-- Pool views (analytics)
CREATE TABLE activity.pool_views (
    pool_id    uuid        NOT NULL REFERENCES data.pools(id)
                           ON UPDATE CASCADE ON DELETE CASCADE,
    user_id    smallint    NOT NULL REFERENCES core.users(id)
                           ON UPDATE CASCADE ON DELETE CASCADE,
    viewed_at  timestamptz NOT NULL DEFAULT statement_timestamp(),

    PRIMARY KEY (pool_id, viewed_at, user_id)
);

-- Tag usage tracking (when tag is used as filter)
CREATE TABLE activity.tag_uses (
    tag_id      uuid        NOT NULL REFERENCES data.tags(id)
                            ON UPDATE CASCADE ON DELETE CASCADE,
    user_id     smallint    NOT NULL REFERENCES core.users(id)
                            ON UPDATE CASCADE ON DELETE CASCADE,
    used_at     timestamptz NOT NULL DEFAULT statement_timestamp(),
    is_included boolean     NOT NULL,  -- true=included in filter, false=excluded

    PRIMARY KEY (tag_id, used_at, user_id)
);

-- Audit log (unified journal for all user actions)
CREATE TABLE activity.audit_log (
    id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id        smallint    NOT NULL REFERENCES core.users(id)
                               ON UPDATE CASCADE ON DELETE RESTRICT,
    action_type_id smallint    NOT NULL REFERENCES activity.action_types(id)
                               ON UPDATE CASCADE ON DELETE RESTRICT,
    object_type_id smallint    REFERENCES core.object_types(id)
                               ON UPDATE CASCADE ON DELETE RESTRICT,
    object_id      uuid,
    details        jsonb,               -- action-specific payload
    performed_at   timestamptz NOT NULL DEFAULT statement_timestamp()
);

-- =============================================================================
-- SEED DATA
-- =============================================================================

-- Object types
INSERT INTO core.object_types (name) VALUES
    ('file'), ('tag'), ('category'), ('pool');

-- Action types
INSERT INTO activity.action_types (name) VALUES
    -- Auth
    ('user_login'), ('user_logout'),
    -- Files
    ('file_create'), ('file_edit'), ('file_delete'), ('file_restore'),
    ('file_permanent_delete'), ('file_replace'),
    -- Tags
    ('tag_create'), ('tag_edit'), ('tag_delete'),
    -- Categories
    ('category_create'), ('category_edit'), ('category_delete'),
    -- Pools
    ('pool_create'), ('pool_edit'), ('pool_delete'),
    -- Relations
    ('file_tag_add'), ('file_tag_remove'),
    ('file_pool_add'), ('file_pool_remove'),
    -- ACL
    ('acl_change'),
    -- Admin
    ('user_create'), ('user_delete'), ('user_block'), ('user_unblock'),
    ('user_role_change'),
    -- Sessions
    ('session_terminate');

-- =============================================================================
-- INDEXES
-- =============================================================================

-- core
CREATE INDEX idx__users__name ON core.users USING hash (name);

-- data.categories
CREATE INDEX idx__categories__creator_id ON data.categories USING hash (creator_id);

-- data.tags
CREATE INDEX idx__tags__category_id ON data.tags USING hash (category_id);
CREATE INDEX idx__tags__creator_id  ON data.tags USING hash (creator_id);

-- data.tag_rules
CREATE INDEX idx__tag_rules__when ON data.tag_rules USING hash (when_tag_id);
CREATE INDEX idx__tag_rules__then ON data.tag_rules USING hash (then_tag_id);

-- data.files
CREATE INDEX idx__files__mime_id           ON data.files USING hash (mime_id);
CREATE INDEX idx__files__creator_id        ON data.files USING hash (creator_id);
CREATE INDEX idx__files__content_datetime  ON data.files USING btree (content_datetime DESC NULLS LAST);
CREATE INDEX idx__files__is_deleted        ON data.files USING btree (is_deleted) WHERE is_deleted = true;
CREATE INDEX idx__files__phash             ON data.files USING btree (phash) WHERE phash IS NOT NULL;

-- data.file_tag
CREATE INDEX idx__file_tag__tag_id  ON data.file_tag USING hash (tag_id);
CREATE INDEX idx__file_tag__file_id ON data.file_tag USING hash (file_id);

-- data.pools
CREATE INDEX idx__pools__creator_id ON data.pools USING hash (creator_id);

-- data.file_pool
CREATE INDEX idx__file_pool__pool_id ON data.file_pool USING hash (pool_id);
CREATE INDEX idx__file_pool__file_id ON data.file_pool USING hash (file_id);

-- acl.permissions
CREATE INDEX idx__acl__object ON acl.permissions USING btree (object_type_id, object_id);
CREATE INDEX idx__acl__user   ON acl.permissions USING hash (user_id);

-- activity.sessions
CREATE INDEX idx__sessions__user_id    ON activity.sessions USING hash (user_id);
CREATE INDEX idx__sessions__token_hash ON activity.sessions USING hash (token_hash);

-- activity.file_views
CREATE INDEX idx__file_views__user_id ON activity.file_views USING hash (user_id);

-- activity.pool_views
CREATE INDEX idx__pool_views__user_id ON activity.pool_views USING hash (user_id);

-- activity.tag_uses
CREATE INDEX idx__tag_uses__user_id ON activity.tag_uses USING hash (user_id);

-- activity.audit_log
CREATE INDEX idx__audit_log__user_id        ON activity.audit_log USING hash (user_id);
CREATE INDEX idx__audit_log__action_type_id ON activity.audit_log USING hash (action_type_id);
CREATE INDEX idx__audit_log__object         ON activity.audit_log USING btree (object_type_id, object_id)
    WHERE object_id IS NOT NULL;
CREATE INDEX idx__audit_log__performed_at   ON activity.audit_log USING btree (performed_at DESC);

-- =============================================================================
-- COMMENTS
-- =============================================================================

COMMENT ON TABLE  core.users              IS 'Application users';
COMMENT ON TABLE  core.mime_types         IS 'Whitelist of supported MIME types';
COMMENT ON TABLE  core.object_types       IS 'Reference: entity types for ACL and audit log';
COMMENT ON TABLE  data.categories         IS 'Logical grouping of tags';
COMMENT ON TABLE  data.tags               IS 'File labels/tags';
COMMENT ON TABLE  data.tag_rules          IS 'Auto-tagging rules: when when_tag is assigned, then_tag follows';
COMMENT ON TABLE  data.files              IS 'Managed files; actual content stored on disk as {id}.{ext}';
COMMENT ON TABLE  data.file_tag           IS 'Many-to-many: files <-> tags';
COMMENT ON TABLE  data.pools              IS 'Ordered collections of files';
COMMENT ON TABLE  data.file_pool          IS 'Many-to-many: files <-> pools, with ordering';
COMMENT ON TABLE  acl.permissions         IS 'Per-object permissions (used when is_public=false)';
COMMENT ON TABLE  activity.action_types   IS 'Reference: types of auditable user actions';
COMMENT ON TABLE  activity.sessions       IS 'Active user sessions';
COMMENT ON TABLE  activity.file_views     IS 'File view history';
COMMENT ON TABLE  activity.pool_views     IS 'Pool view history';
COMMENT ON TABLE  activity.tag_uses       IS 'Tag usage in filters';
COMMENT ON TABLE  activity.audit_log      IS 'Unified audit trail for all user actions';

COMMENT ON COLUMN data.files.original_name    IS 'Original filename at upload time';
COMMENT ON COLUMN data.files.content_datetime IS 'Content datetime (e.g. when photo was taken); falls back to EXIF DateTimeOriginal';
COMMENT ON COLUMN data.files.metadata         IS 'User-editable key-value metadata';
COMMENT ON COLUMN data.files.exif             IS 'EXIF data extracted at upload time (immutable, system-managed)';
COMMENT ON COLUMN data.files.phash            IS 'Perceptual hash for image/video duplicate detection';
COMMENT ON COLUMN data.files.is_deleted       IS 'Soft-deleted files (trash); true = in recycle bin';
COMMENT ON COLUMN data.file_pool.position     IS 'Manual ordering within pool; uses gapped integers';
