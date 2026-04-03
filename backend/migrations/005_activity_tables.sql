-- +goose Up

CREATE TABLE activity.action_types (
    id   smallint    GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name varchar(64) NOT NULL,

    CONSTRAINT uni__action_types__name UNIQUE (name)
);

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

CREATE TABLE activity.file_views (
    file_id    uuid        NOT NULL REFERENCES data.files(id)
                           ON UPDATE CASCADE ON DELETE CASCADE,
    user_id    smallint    NOT NULL REFERENCES core.users(id)
                           ON UPDATE CASCADE ON DELETE CASCADE,
    viewed_at  timestamptz NOT NULL DEFAULT statement_timestamp(),

    PRIMARY KEY (file_id, viewed_at, user_id)
);

CREATE TABLE activity.pool_views (
    pool_id    uuid        NOT NULL REFERENCES data.pools(id)
                           ON UPDATE CASCADE ON DELETE CASCADE,
    user_id    smallint    NOT NULL REFERENCES core.users(id)
                           ON UPDATE CASCADE ON DELETE CASCADE,
    viewed_at  timestamptz NOT NULL DEFAULT statement_timestamp(),

    PRIMARY KEY (pool_id, viewed_at, user_id)
);

CREATE TABLE activity.tag_uses (
    tag_id      uuid        NOT NULL REFERENCES data.tags(id)
                            ON UPDATE CASCADE ON DELETE CASCADE,
    user_id     smallint    NOT NULL REFERENCES core.users(id)
                            ON UPDATE CASCADE ON DELETE CASCADE,
    used_at     timestamptz NOT NULL DEFAULT statement_timestamp(),
    is_included boolean     NOT NULL,  -- true=included in filter, false=excluded

    PRIMARY KEY (tag_id, used_at, user_id)
);

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

COMMENT ON TABLE activity.action_types IS 'Reference: types of auditable user actions';
COMMENT ON TABLE activity.sessions     IS 'Active user sessions';
COMMENT ON TABLE activity.file_views   IS 'File view history';
COMMENT ON TABLE activity.pool_views   IS 'Pool view history';
COMMENT ON TABLE activity.tag_uses     IS 'Tag usage in filters';
COMMENT ON TABLE activity.audit_log    IS 'Unified audit trail for all user actions';

-- +goose Down

DROP TABLE IF EXISTS activity.audit_log;
DROP TABLE IF EXISTS activity.tag_uses;
DROP TABLE IF EXISTS activity.pool_views;
DROP TABLE IF EXISTS activity.file_views;
DROP TABLE IF EXISTS activity.sessions;
DROP TABLE IF EXISTS activity.action_types;
