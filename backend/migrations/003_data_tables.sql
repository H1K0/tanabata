-- +goose Up

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

CREATE TABLE data.tag_rules (
    when_tag_id uuid    NOT NULL REFERENCES data.tags(id)
                        ON UPDATE CASCADE ON DELETE CASCADE,
    then_tag_id uuid    NOT NULL REFERENCES data.tags(id)
                        ON UPDATE CASCADE ON DELETE CASCADE,
    is_active   boolean NOT NULL DEFAULT true,

    PRIMARY KEY (when_tag_id, then_tag_id)
);

CREATE TABLE data.files (
    id               uuid         NOT NULL DEFAULT public.uuid_v7() PRIMARY KEY,
    original_name    varchar(256),          -- original filename at upload time
    mime_id          smallint     NOT NULL REFERENCES core.mime_types(id)
                                  ON UPDATE CASCADE ON DELETE RESTRICT,
    content_datetime timestamptz  NOT NULL DEFAULT clock_timestamp(),  -- content datetime (e.g. photo taken)
    notes            text,
    metadata         jsonb,                 -- user-editable key-value data
    exif             jsonb,                 -- EXIF data extracted at upload (immutable)
    phash            bigint,                -- perceptual hash for duplicate detection (future)
    creator_id       smallint     NOT NULL REFERENCES core.users(id)
                                  ON UPDATE CASCADE ON DELETE RESTRICT,
    is_public        boolean      NOT NULL DEFAULT false,
    is_deleted       boolean      NOT NULL DEFAULT false  -- soft delete (trash)
);

CREATE TABLE data.file_tag (
    file_id uuid NOT NULL REFERENCES data.files(id)
                 ON UPDATE CASCADE ON DELETE CASCADE,
    tag_id  uuid NOT NULL REFERENCES data.tags(id)
                 ON UPDATE CASCADE ON DELETE CASCADE,

    PRIMARY KEY (file_id, tag_id)
);

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

COMMENT ON TABLE  data.categories IS 'Logical grouping of tags';
COMMENT ON TABLE  data.tags       IS 'File labels/tags';
COMMENT ON TABLE  data.tag_rules  IS 'Auto-tagging rules: when when_tag is assigned, then_tag follows';
COMMENT ON TABLE  data.files      IS 'Managed files; actual content stored on disk as {id}.{ext}';
COMMENT ON TABLE  data.file_tag   IS 'Many-to-many: files <-> tags';
COMMENT ON TABLE  data.pools      IS 'Ordered collections of files';
COMMENT ON TABLE  data.file_pool  IS 'Many-to-many: files <-> pools, with ordering';

COMMENT ON COLUMN data.files.original_name    IS 'Original filename at upload time';
COMMENT ON COLUMN data.files.content_datetime IS 'Content datetime (e.g. when photo was taken); falls back to EXIF DateTimeOriginal';
COMMENT ON COLUMN data.files.metadata         IS 'User-editable key-value metadata';
COMMENT ON COLUMN data.files.exif             IS 'EXIF data extracted at upload time (immutable, system-managed)';
COMMENT ON COLUMN data.files.phash            IS 'Perceptual hash for image/video duplicate detection';
COMMENT ON COLUMN data.files.is_deleted       IS 'Soft-deleted files (trash); true = in recycle bin';
COMMENT ON COLUMN data.file_pool.position     IS 'Manual ordering within pool; uses gapped integers';

-- +goose Down

DROP TABLE IF EXISTS data.file_pool;
DROP TABLE IF EXISTS data.pools;
DROP TABLE IF EXISTS data.file_tag;
DROP TABLE IF EXISTS data.files;
DROP TABLE IF EXISTS data.tag_rules;
DROP TABLE IF EXISTS data.tags;
DROP TABLE IF EXISTS data.categories;
