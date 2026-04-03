-- +goose Up

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

COMMENT ON TABLE acl.permissions IS 'Per-object permissions (used when is_public=false)';

-- +goose Down

DROP TABLE IF EXISTS acl.permissions;
