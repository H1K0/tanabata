-- +goose Up

CREATE TABLE core.users (
    id         smallint    GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       varchar(32) NOT NULL,
    password   text        NOT NULL,  -- bcrypt hash via pgcrypto
    is_admin   boolean     NOT NULL DEFAULT false,
    can_create boolean     NOT NULL DEFAULT false,
    is_blocked boolean     NOT NULL DEFAULT false,

    CONSTRAINT uni__users__name UNIQUE (name)
);

CREATE TABLE core.mime_types (
    id        smallint     GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name      varchar(127) NOT NULL,
    extension varchar(16)  NOT NULL,

    CONSTRAINT uni__mime_types__name UNIQUE (name)
);

CREATE TABLE core.object_types (
    id   smallint    GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name varchar(32) NOT NULL,

    CONSTRAINT uni__object_types__name UNIQUE (name)
);

COMMENT ON TABLE core.users       IS 'Application users';
COMMENT ON TABLE core.mime_types  IS 'Whitelist of supported MIME types';
COMMENT ON TABLE core.object_types IS 'Reference: entity types for ACL and audit log';

-- +goose Down

DROP TABLE IF EXISTS core.object_types;
DROP TABLE IF EXISTS core.mime_types;
DROP TABLE IF EXISTS core.users;
