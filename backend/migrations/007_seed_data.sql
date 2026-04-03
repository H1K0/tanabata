-- +goose Up

INSERT INTO core.object_types (name) VALUES
    ('file'), ('tag'), ('category'), ('pool');

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

INSERT INTO core.users (name, password, is_admin, can_create) VALUES
    ('admin', '$2a$10$zk.VTFjRRxbkTE7cKfc7KOWeZfByk1VEkbkgZMJggI1fFf.yDEHZy', true, true);

-- +goose Down

DELETE FROM core.users WHERE name = 'admin';
DELETE FROM activity.action_types;
DELETE FROM core.object_types;
