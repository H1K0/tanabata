-- +goose Up

INSERT INTO core.mime_types (name, extension) VALUES
    ('image/jpeg',       'jpg'),
    ('image/png',        'png'),
    ('image/gif',        'gif'),
    ('image/webp',       'webp'),
    ('video/mp4',        'mp4'),
    ('video/quicktime',  'mov'),
    ('video/x-msvideo',  'avi'),
    ('video/webm',       'webm'),
    ('video/3gpp',       '3gp'),
    ('video/x-m4v',      'm4v');

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

-- The initial administrator is created at application startup from the
-- ADMIN_USERNAME / ADMIN_PASSWORD environment variables (see UserService.
-- EnsureAdmin), so no default credentials are seeded here.

-- +goose Down

DELETE FROM activity.action_types;
DELETE FROM core.object_types;
DELETE FROM core.mime_types;
