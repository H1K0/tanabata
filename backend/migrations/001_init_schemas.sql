-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE SCHEMA IF NOT EXISTS core;
CREATE SCHEMA IF NOT EXISTS data;
CREATE SCHEMA IF NOT EXISTS acl;
CREATE SCHEMA IF NOT EXISTS activity;

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

-- +goose Down

DROP FUNCTION IF EXISTS public.uuid_extract_timestamp(uuid);
DROP FUNCTION IF EXISTS public.uuid_v7(timestamptz);

DROP SCHEMA IF EXISTS activity;
DROP SCHEMA IF EXISTS acl;
DROP SCHEMA IF EXISTS data;
DROP SCHEMA IF EXISTS core;

DROP EXTENSION IF EXISTS "uuid-ossp";
DROP EXTENSION IF EXISTS pgcrypto;
