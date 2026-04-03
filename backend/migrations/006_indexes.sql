-- +goose Up

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

-- +goose Down

DROP INDEX IF EXISTS activity.idx__audit_log__performed_at;
DROP INDEX IF EXISTS activity.idx__audit_log__object;
DROP INDEX IF EXISTS activity.idx__audit_log__action_type_id;
DROP INDEX IF EXISTS activity.idx__audit_log__user_id;
DROP INDEX IF EXISTS activity.idx__tag_uses__user_id;
DROP INDEX IF EXISTS activity.idx__pool_views__user_id;
DROP INDEX IF EXISTS activity.idx__file_views__user_id;
DROP INDEX IF EXISTS activity.idx__sessions__token_hash;
DROP INDEX IF EXISTS activity.idx__sessions__user_id;
DROP INDEX IF EXISTS acl.idx__acl__user;
DROP INDEX IF EXISTS acl.idx__acl__object;
DROP INDEX IF EXISTS data.idx__file_pool__file_id;
DROP INDEX IF EXISTS data.idx__file_pool__pool_id;
DROP INDEX IF EXISTS data.idx__pools__creator_id;
DROP INDEX IF EXISTS data.idx__file_tag__file_id;
DROP INDEX IF EXISTS data.idx__file_tag__tag_id;
DROP INDEX IF EXISTS data.idx__files__phash;
DROP INDEX IF EXISTS data.idx__files__is_deleted;
DROP INDEX IF EXISTS data.idx__files__content_datetime;
DROP INDEX IF EXISTS data.idx__files__creator_id;
DROP INDEX IF EXISTS data.idx__files__mime_id;
DROP INDEX IF EXISTS data.idx__tag_rules__then;
DROP INDEX IF EXISTS data.idx__tag_rules__when;
DROP INDEX IF EXISTS data.idx__tags__creator_id;
DROP INDEX IF EXISTS data.idx__tags__category_id;
DROP INDEX IF EXISTS data.idx__categories__creator_id;
DROP INDEX IF EXISTS core.idx__users__name;
