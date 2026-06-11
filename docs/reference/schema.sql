--
-- PostgreSQL database dump
--

-- Dumped from database version 14.20 (Ubuntu 14.20-1.pgdg22.04+1)
-- Dumped by pg_dump version 17.4

-- Started on 2026-03-31 00:31:48

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- TOC entry 7 (class 2615 OID 2200)
-- Name: public; Type: SCHEMA; Schema: -; Owner: hiko
--

-- *not* creating schema, since initdb creates it


ALTER SCHEMA public OWNER TO hiko;

--
-- TOC entry 2 (class 3079 OID 16486)
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- TOC entry 3596 (class 0 OID 0)
-- Dependencies: 2
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- TOC entry 3 (class 3079 OID 16475)
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- TOC entry 3597 (class 0 OID 0)
-- Dependencies: 3
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- TOC entry 987 (class 1247 OID 28864)
-- Name: file; Type: TYPE; Schema: public; Owner: hiko
--

CREATE TYPE public.file AS (
	id uuid,
	mime_id uuid,
	mime_name character varying(127),
	extension character varying(16),
	orig_name character varying(256),
	datetime timestamp with time zone,
	notes character varying(1024),
	created timestamp with time zone,
	creator_id uuid,
	creator_name character varying(32),
	is_private boolean
);


ALTER TYPE public.file OWNER TO hiko;

--
-- TOC entry 315 (class 1255 OID 17507)
-- Name: get_column_names(name, name); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.get_column_names(table_n name, schema_n name DEFAULT 'public'::name) RETURNS SETOF name
    LANGUAGE sql SECURITY DEFINER
    AS $$
SELECT column_name FROM information_schema.columns WHERE table_name = table_n AND table_schema = schema_n;
$$;


ALTER FUNCTION public.get_column_names(table_n name, schema_n name) OWNER TO hiko;

--
-- TOC entry 322 (class 1255 OID 17546)
-- Name: tfm__add_file_to_tag_recursive(uuid, uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm__add_file_to_tag_recursive(f_id uuid, t_id uuid) RETURNS SETOF uuid
    LANGUAGE plpgsql
    AS $$
DECLARE
	tmp uuid;
	pt_id uuid;
	ppt_id uuid;
BEGIN
	INSERT INTO file_tag VALUES (f_id, t_id) ON CONFLICT DO NOTHING RETURNING tag_id INTO tmp;
	IF tmp IS NULL THEN
		RETURN;
	END IF;
	RETURN NEXT t_id;
	FOR pt_id IN
		SELECT a.parent_id FROM autotags a WHERE a.child_id=t_id AND a.is_active
	LOOP
		FOR ppt_id IN SELECT tfm__add_file_to_tag_recursive(f_id, pt_id)
		LOOP
			RETURN NEXT ppt_id;
		END LOOP;
	END LOOP;
END;
$$;


ALTER FUNCTION public.tfm__add_file_to_tag_recursive(f_id uuid, t_id uuid) OWNER TO hiko;

--
-- TOC entry 323 (class 1255 OID 17539)
-- Name: tfm_add_autotag(uuid, uuid, uuid, boolean, boolean); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_add_autotag(s_id uuid, tc_id uuid, tp_id uuid, is_active boolean DEFAULT NULL::boolean, apply_to_existing boolean DEFAULT NULL::boolean) RETURNS boolean
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
	ct_id uuid;
	pt_id uuid;
	f_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	PERFORM FROM tags t WHERE t.id=tc_id AND (NOT t.is_private OR t.creator_id=u_id);
	IF NOT FOUND THEN RAISE 'Invalid child tag id'; END IF;
	PERFORM FROM tags t WHERE t.id=tp_id AND (NOT t.is_private OR t.creator_id=u_id);
	IF NOT FOUND THEN RAISE 'Invalid parent tag id'; END IF;
	EXECUTE 'INSERT INTO autotags(child_id, parent_id, is_active) VALUES (' ||
		quote_literal(tc_id) || ',' ||
		quote_literal(tp_id) || ',' ||
		CASE WHEN is_active IS NULL THEN 'DEFAULT' ELSE quote_literal(is_active) END ||
	') ON CONFLICT DO NOTHING RETURNING child_id, parent_id' INTO ct_id, pt_id;
	IF ct_id IS NOT NULL AND coalesce(apply_to_existing, true) THEN
		FOR f_id IN
			SELECT ft.file_id FROM file_tag ft WHERE ft.tag_id=tc_id
		LOOP
			PERFORM tfm__add_file_to_tag_recursive(f_id, tp_id);
		END LOOP;
	END IF;
	RETURN (ct_id IS NOT NULL);
END;
$$;


ALTER FUNCTION public.tfm_add_autotag(s_id uuid, tc_id uuid, tp_id uuid, is_active boolean, apply_to_existing boolean) OWNER TO hiko;

--
-- TOC entry 298 (class 1255 OID 17380)
-- Name: tfm_add_category(uuid, character varying, character varying, character, boolean); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_add_category(s_id uuid, c_name character varying, c_notes character varying DEFAULT NULL::character varying, c_color character DEFAULT NULL::bpchar, c_is_private boolean DEFAULT NULL::boolean, OUT c_id uuid) RETURNS uuid
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	EXECUTE 'INSERT INTO categories(name, notes, color, creator_id, is_private) VALUES(' ||
		quote_literal(c_name) || ',' ||
		CASE WHEN c_notes IS NULL THEN 'DEFAULT' ELSE quote_literal(c_notes) END || ',' ||
		CASE WHEN c_color IS NULL THEN 'DEFAULT' ELSE quote_literal(c_color) END || ',' ||
		quote_literal(u_id) || ',' ||
		CASE WHEN c_is_private IS NULL THEN 'DEFAULT' ELSE quote_literal(c_is_private) END ||
	') RETURNING id' INTO c_id;
END;
$$;


ALTER FUNCTION public.tfm_add_category(s_id uuid, c_name character varying, c_notes character varying, c_color character, c_is_private boolean, OUT c_id uuid) OWNER TO hiko;

--
-- TOC entry 330 (class 1255 OID 120220)
-- Name: tfm_add_file(uuid, character varying, timestamp with time zone, character varying, boolean, character varying, jsonb); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_add_file(s_id uuid, f_mime character varying, f_datetime timestamp with time zone DEFAULT NULL::timestamp with time zone, f_notes character varying DEFAULT NULL::character varying, f_is_private boolean DEFAULT NULL::boolean, f_orig_name character varying DEFAULT NULL::character varying, f_metadata jsonb DEFAULT NULL::jsonb, OUT f_id uuid, OUT ext character varying) RETURNS record
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
	m_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	SELECT m.id, m.extension FROM mime m WHERE m.name=f_mime INTO m_id, ext;
	IF m_id IS NULL THEN RAISE 'Unsupported MIME: %', f_mime; END IF;
	EXECUTE 'INSERT INTO files(mime_id, datetime, notes, creator_id, is_private, orig_name, metadata) VALUES(' ||
		quote_literal(m_id) || ',' ||
		CASE WHEN f_datetime IS NULL THEN 'DEFAULT' ELSE quote_literal(f_datetime) END || ',' ||
		CASE WHEN f_notes IS NULL THEN 'DEFAULT' ELSE quote_literal(f_notes) END || ',' ||
		quote_literal(u_id) || ',' ||
		CASE WHEN f_is_private IS NULL THEN 'DEFAULT' ELSE quote_literal(f_is_private) END || ',' ||
		CASE WHEN f_orig_name IS NULL THEN 'DEFAULT' ELSE quote_literal(f_orig_name) END || ',' ||
		CASE WHEN f_metadata IS NULL THEN 'DEFAULT' ELSE quote_literal(f_metadata) END ||
	') RETURNING id' INTO f_id;
END;
$$;


ALTER FUNCTION public.tfm_add_file(s_id uuid, f_mime character varying, f_datetime timestamp with time zone, f_notes character varying, f_is_private boolean, f_orig_name character varying, f_metadata jsonb, OUT f_id uuid, OUT ext character varying) OWNER TO hiko;

--
-- TOC entry 318 (class 1255 OID 17527)
-- Name: tfm_add_file_to_pool(uuid, uuid, uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_add_file_to_pool(s_id uuid, f_id uuid, p_id uuid) RETURNS boolean
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
	tmp uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	PERFORM FROM files f WHERE f.id=f_id AND (NOT f.is_private OR f.creator_id=u_id);
	IF NOT FOUND THEN RAISE 'Invalid file id'; END IF;
	PERFORM FROM pools p WHERE p.id=p_id AND (NOT p.is_private OR p.creator_id=u_id);
	IF NOT FOUND THEN RAISE 'Invalid pool id'; END IF;
	INSERT INTO file_pool VALUES (f_id, p_id) ON CONFLICT DO NOTHING RETURNING pool_id INTO tmp;
	RETURN (tmp IS NOT NULL);
END;
$$;


ALTER FUNCTION public.tfm_add_file_to_pool(s_id uuid, f_id uuid, p_id uuid) OWNER TO hiko;

--
-- TOC entry 309 (class 1255 OID 17461)
-- Name: tfm_add_file_to_tag(uuid, uuid, uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_add_file_to_tag(s_id uuid, f_id uuid, t_id uuid) RETURNS SETOF uuid
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	PERFORM FROM files f WHERE f.id=f_id AND (NOT f.is_private OR f.creator_id=u_id);
	IF NOT FOUND THEN RAISE 'Invalid file id'; END IF;
	PERFORM FROM tags t WHERE t.id=t_id AND (NOT t.is_private OR t.creator_id=u_id);
	IF NOT FOUND THEN RAISE 'Invalid tag id'; END IF;
	RETURN QUERY SELECT tfm__add_file_to_tag_recursive(f_id, t_id);
END;
$$;


ALTER FUNCTION public.tfm_add_file_to_tag(s_id uuid, f_id uuid, t_id uuid) OWNER TO hiko;

--
-- TOC entry 301 (class 1255 OID 17397)
-- Name: tfm_add_pool(uuid, character varying, character varying, uuid, boolean); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_add_pool(s_id uuid, p_name character varying, p_notes character varying DEFAULT NULL::character varying, p_parent_id uuid DEFAULT NULL::uuid, p_is_private boolean DEFAULT NULL::boolean, OUT p_id uuid) RETURNS uuid
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	EXECUTE 'INSERT INTO pools(name, notes, parent_id, creator_id, is_private) VALUES(' ||
		quote_literal(p_name) || ',' ||
		CASE WHEN p_notes IS NULL THEN 'DEFAULT' ELSE quote_literal(p_notes) END || ',' ||
		CASE WHEN p_parent_id IS NULL THEN 'DEFAULT' ELSE quote_literal(p_parent_id) END || ',' ||
		quote_literal(u_id) || ',' ||
		CASE WHEN p_is_private IS NULL THEN 'DEFAULT' ELSE quote_literal(p_is_private) END ||
	') RETURNING id' INTO p_id;
END;
$$;


ALTER FUNCTION public.tfm_add_pool(s_id uuid, p_name character varying, p_notes character varying, p_parent_id uuid, p_is_private boolean, OUT p_id uuid) OWNER TO hiko;

--
-- TOC entry 305 (class 1255 OID 17381)
-- Name: tfm_add_tag(uuid, character varying, character varying, character, uuid, boolean); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_add_tag(s_id uuid, t_name character varying, t_notes character varying DEFAULT NULL::character varying, t_color character DEFAULT NULL::bpchar, t_category_id uuid DEFAULT NULL::uuid, t_is_private boolean DEFAULT NULL::boolean, OUT t_id uuid) RETURNS uuid
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	EXECUTE 'INSERT INTO tags(name, notes, color, category_id, creator_id, is_private) VALUES(' ||
		quote_literal(t_name) || ',' ||
		CASE WHEN t_notes IS NULL THEN 'DEFAULT' ELSE quote_literal(t_notes) END || ',' ||
		CASE WHEN t_color IS NULL THEN 'DEFAULT' ELSE quote_literal(t_color) END || ',' ||
		CASE WHEN t_category_id IS NULL THEN 'DEFAULT' ELSE quote_literal(t_category_id) END || ',' ||
		quote_literal(u_id) || ',' ||
		CASE WHEN t_is_private IS NULL THEN 'DEFAULT' ELSE quote_literal(t_is_private) END ||
	') RETURNING id' INTO t_id;
END;
$$;


ALTER FUNCTION public.tfm_add_tag(s_id uuid, t_name character varying, t_notes character varying, t_color character, t_category_id uuid, t_is_private boolean, OUT t_id uuid) OWNER TO hiko;

--
-- TOC entry 312 (class 1255 OID 17497)
-- Name: tfm_edit_category(uuid, uuid, character varying, character varying, character, boolean); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_edit_category(IN s_id uuid, IN c_id uuid, IN c_name character varying DEFAULT NULL::character varying, IN c_notes character varying DEFAULT NULL::character varying, IN c_color character DEFAULT NULL::bpchar, IN c_is_private boolean DEFAULT NULL::boolean)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT c.is_private OR c.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM categories c WHERE c.id=c_id)
	THEN RAISE 'Not allowed'; END IF;
	UPDATE categories SET
		name = coalesce(c_name, name),
		notes = coalesce(c_notes, notes),
		color = CASE WHEN c_color=''::character THEN NULL ELSE coalesce(c_color, color) END,
		is_private = coalesce(c_is_private, is_private)
	WHERE id=c_id;
END;
$$;


ALTER PROCEDURE public.tfm_edit_category(IN s_id uuid, IN c_id uuid, IN c_name character varying, IN c_notes character varying, IN c_color character, IN c_is_private boolean) OWNER TO hiko;

--
-- TOC entry 319 (class 1255 OID 17500)
-- Name: tfm_edit_file(uuid, uuid, character varying, timestamp with time zone, character varying, boolean); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_edit_file(IN s_id uuid, IN f_id uuid, IN f_mime_name character varying DEFAULT NULL::character varying, IN f_datetime timestamp with time zone DEFAULT NULL::timestamp with time zone, IN f_notes character varying DEFAULT NULL::character varying, IN f_is_private boolean DEFAULT NULL::boolean)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$DECLARE
	u_id uuid;
	m_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT f.is_private OR f.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM files f WHERE f.id=f_id)
	THEN RAISE 'Not allowed'; END IF;
	IF f_mime_name IS NOT NULL THEN
		SELECT m.id FROM mime m WHERE m.name=f_mime_name INTO m_id;
		IF m_id IS NULL THEN RAISE 'Unsupported MIME'; END IF;
	END IF;
	UPDATE files SET
		mime_id = coalesce(m_id, mime_id),
		datetime = coalesce(f_datetime, datetime),
		notes = coalesce(f_notes, notes),
		is_private = coalesce(f_is_private, is_private)
	WHERE id=f_id;
END;
$$;


ALTER PROCEDURE public.tfm_edit_file(IN s_id uuid, IN f_id uuid, IN f_mime_name character varying, IN f_datetime timestamp with time zone, IN f_notes character varying, IN f_is_private boolean) OWNER TO hiko;

--
-- TOC entry 311 (class 1255 OID 17501)
-- Name: tfm_edit_pool(uuid, uuid, character varying, character varying, uuid, boolean); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_edit_pool(IN s_id uuid, IN p_id uuid, IN p_name character varying DEFAULT NULL::character varying, IN p_notes character varying DEFAULT NULL::character varying, IN p_parent_id uuid DEFAULT NULL::uuid, IN p_is_private boolean DEFAULT NULL::boolean)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT p.is_private OR p.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM pools p WHERE p.id=p_id)
	THEN RAISE 'Not allowed'; END IF;
	UPDATE pools p SET
		p.name = coalesce(p_name, p.name),
		p.notes = coalesce(p_notes, p.notes),
		p.parent_id = CASE WHEN p_parent_id=uuid_nil() THEN NULL ELSE coalesce(p_parent_id, p.parent_id) END,
		p.is_private = coalesce(p_is_private, p.is_private)
	WHERE p.id=p_id;
END;
$$;


ALTER PROCEDURE public.tfm_edit_pool(IN s_id uuid, IN p_id uuid, IN p_name character varying, IN p_notes character varying, IN p_parent_id uuid, IN p_is_private boolean) OWNER TO hiko;

--
-- TOC entry 320 (class 1255 OID 17502)
-- Name: tfm_edit_tag(uuid, uuid, character varying, character varying, character, uuid, boolean); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_edit_tag(IN s_id uuid, IN t_id uuid, IN t_name character varying DEFAULT NULL::character varying, IN t_notes character varying DEFAULT NULL::character varying, IN t_color character DEFAULT NULL::bpchar, IN t_category_id uuid DEFAULT NULL::uuid, IN t_is_private boolean DEFAULT NULL::boolean)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT t.is_private OR t.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM tags t WHERE t.id=t_id)
	THEN RAISE 'Not allowed'; END IF;
	UPDATE tags SET
		name = coalesce(t_name, name),
		notes = coalesce(t_notes, notes),
		color = CASE WHEN t_color=''::character THEN NULL ELSE coalesce(t_color, color) END,
		category_id = CASE WHEN t_category_id=uuid_nil() THEN NULL ELSE coalesce(t_category_id, category_id) END,
		is_private = coalesce(t_is_private, is_private)
	WHERE id=t_id;
END;
$$;


ALTER PROCEDURE public.tfm_edit_tag(IN s_id uuid, IN t_id uuid, IN t_name character varying, IN t_notes character varying, IN t_color character, IN t_category_id uuid, IN t_is_private boolean) OWNER TO hiko;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 220 (class 1259 OID 17064)
-- Name: autotags; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.autotags (
    parent_id uuid NOT NULL,
    child_id uuid NOT NULL,
    is_active boolean DEFAULT true NOT NULL
);


ALTER TABLE public.autotags OWNER TO hiko;

--
-- TOC entry 228 (class 1259 OID 17474)
-- Name: v_autotags; Type: VIEW; Schema: public; Owner: hiko
--

CREATE VIEW public.v_autotags AS
 SELECT a.child_id,
    a.parent_id,
    a.is_active
   FROM public.autotags a;


ALTER VIEW public.v_autotags OWNER TO hiko;

--
-- TOC entry 307 (class 1255 OID 17478)
-- Name: tfm_get_autotags(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_autotags(s_id uuid) RETURNS SETOF public.v_autotags
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
	u_is_admin boolean;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	SELECT u.is_admin FROM users u WHERE u.id=u_id INTO u_is_admin;
	RETURN QUERY
	SELECT a.* FROM v_autotags a
	JOIN tags tc ON a.child_id=tc.id AND (NOT tc.is_private OR tc.creator_id=u_id OR u_is_admin)
	JOIN tags tp ON a.parent_id=tp.id AND (NOT tp.is_private OR tp.creator_id=u_id OR u_is_admin);
END;
$$;


ALTER FUNCTION public.tfm_get_autotags(s_id uuid) OWNER TO hiko;

--
-- TOC entry 331 (class 1255 OID 139011)
-- Name: uuid_v7(timestamp with time zone); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.uuid_v7(cts timestamp with time zone DEFAULT clock_timestamp()) RETURNS uuid
    LANGUAGE plpgsql
    AS $$
DECLARE
  state text = current_setting('uuidv7.old_tp',true);
  old_tp text = split_part(state, ':',1);
  base int = coalesce(nullif(split_part(state,':',4),'')::int,(random()*16777215/2-1)::int);
  tp text;
  entropy text;
  seq text=base;
  seqn int=split_part(state,':',2);
  ver text = coalesce(split_part(state,':',3),to_hex(8+(random()*3)::int));
BEGIN
  base = (random()*16777215/2-1)::int;
  tp = lpad(to_hex(floor(extract(epoch from cts)*1000)::int8),12,'0')||'7';
  if tp is distinct from old_tp then
    old_tp = tp;
    ver = to_hex(8+(random()*3)::int);
    base = (random()*16777215/2-1)::int;
    seqn = base;
  else
    seqn = seqn+(random()*1000)::int;
  end if;
  perform set_config('uuidv7.old_tp',old_tp||':'||seqn||':'||ver||':'||base, false);
  entropy = md5(gen_random_uuid()::text);
  seq = lpad(to_hex(seqn),6,'0');
  return (tp || substring(seq from 1 for 3) || ver || substring(seq from 4 for 3) ||
                substring(entropy from 1 for 12))::uuid;
END
$$;


ALTER FUNCTION public.uuid_v7(cts timestamp with time zone) OWNER TO hiko;

--
-- TOC entry 222 (class 1259 OID 17164)
-- Name: categories; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.categories (
    id uuid DEFAULT public.uuid_v7() NOT NULL,
    name character varying(256) NOT NULL,
    notes character varying(1024) DEFAULT ''::character varying NOT NULL,
    color character(6),
    created timestamp with time zone DEFAULT statement_timestamp() NOT NULL,
    creator_id uuid NOT NULL,
    is_private boolean DEFAULT true NOT NULL
);


ALTER TABLE public.categories OWNER TO hiko;

--
-- TOC entry 213 (class 1259 OID 16523)
-- Name: users; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(32) NOT NULL,
    password text NOT NULL,
    is_admin boolean DEFAULT false NOT NULL,
    can_edit boolean DEFAULT false NOT NULL
);


ALTER TABLE public.users OWNER TO hiko;

--
-- TOC entry 226 (class 1259 OID 17441)
-- Name: v_categories; Type: VIEW; Schema: public; Owner: hiko
--

CREATE VIEW public.v_categories AS
 SELECT c.id,
    c.name,
    c.notes,
    c.color,
    c.created,
    c.creator_id,
    u.name AS creator_name,
    c.is_private
   FROM (public.categories c
     JOIN public.users u ON ((c.creator_id = u.id)));


ALTER VIEW public.v_categories OWNER TO hiko;

--
-- TOC entry 299 (class 1255 OID 17456)
-- Name: tfm_get_categories(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_categories(s_id uuid) RETURNS SETOF public.v_categories
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	RETURN QUERY
	SELECT c.* FROM v_categories c
	WHERE NOT c.is_private OR c.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id);
END;
$$;


ALTER FUNCTION public.tfm_get_categories(s_id uuid) OWNER TO hiko;

--
-- TOC entry 328 (class 1255 OID 28865)
-- Name: tfm_get_files(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_files(s_id uuid) RETURNS SETOF public.file
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	RETURN QUERY
	SELECT
		f.id,
		f.mime_id,
		m.name AS mime_name,
		m.extension,
		f.orig_name,
		f.datetime,
		f.notes,
		f.created,
		f.creator_id,
		u.name AS creator_name,
		f.is_private
	FROM files f
	JOIN mime m ON f.mime_id = m.id
	JOIN users u ON f.creator_id = u.id
	WHERE NOT f.is_private OR f.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id);
END;
$$;


ALTER FUNCTION public.tfm_get_files(s_id uuid) OWNER TO hiko;

--
-- TOC entry 288 (class 1255 OID 17615)
-- Name: tfm_get_files_1(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_files_1(s_id uuid) RETURNS SETOF record
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	RETURN QUERY
	SELECT f.* FROM v_files f
	WHERE NOT f.is_private OR f.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id);
END;
$$;


ALTER FUNCTION public.tfm_get_files_1(s_id uuid) OWNER TO hiko;

--
-- TOC entry 329 (class 1255 OID 28866)
-- Name: tfm_get_files_by_filter(uuid, character varying[]); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_files_by_filter(s_id uuid, filters character varying[] DEFAULT NULL::character varying[]) RETURNS SETOF public.file
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
	query_text text = 'SELECT * FROM (SELECT DISTINCT f.*, array_agg(ft.tag_id) OVER (PARTITION BY f.id) AS tags_list FROM tfm_get_files(' || quote_literal(s_id) || ') f ' || 
	                  'LEFT JOIN file_tag ft ON ft.file_id=f.id) _f ' ||
					  'WHERE';
	_filter character varying;
	tmp text;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	RAISE NOTICE '%', filters IS NULL;
	IF filters IS NULL OR cardinality(filters) = 0 THEN
		RETURN QUERY SELECT * FROM tfm_get_files(s_id);
		RETURN;
	END IF;
	-- there was ',tags_list' in the end of columns to select
	query_text := 'SELECT id,mime_id,mime_name,extension,orig_name,datetime,notes,created,creator_id,creator_name,is_private FROM (SELECT DISTINCT f.*, array_agg(ft.tag_id) OVER (PARTITION BY f.id) AS tags_list FROM tfm_get_files(' || quote_literal(s_id) || ') f ' || 
	                  'LEFT JOIN file_tag ft ON ft.file_id=f.id) _f ' ||
					  'WHERE';
	FOREACH _filter IN ARRAY filters LOOP
		IF _filter IN ('(', ')') THEN
			query_text := query_text || _filter;
		ELSIF _filter='&' THEN
			query_text := query_text || ' AND';
		ELSIF _filter='|' THEN
			query_text := query_text || ' OR';
		ELSIF _filter='!' THEN
			query_text := query_text || ' NOT';
		ELSIF _filter='t=' || uuid_nil() THEN
			query_text := query_text || ' (tags_list=array[NULL]::uuid[] OR ''d6d8129a-984d-4451-8c83-d04523ced8a8''=ANY(tags_list))';
		ELSIF _filter LIKE 't=%' THEN
			query_text := query_text || ' ' || quote_literal(substring(_filter, 3)) || '=ANY(tags_list)';
		ELSIF _filter LIKE 'm=%' THEN
			query_text := query_text || ' mime_id=' || quote_literal(substring(_filter, 3));
		ELSIF _filter LIKE 'm~%' THEN
			query_text := query_text || ' mime_name LIKE ' || quote_literal(substring(_filter, 3));
		ELSE
			RAISE 'Invalid condition';
		END IF;
	END LOOP;
	RETURN QUERY EXECUTE query_text;
END;
$$;


ALTER FUNCTION public.tfm_get_files_by_filter(s_id uuid, filters character varying[]) OWNER TO hiko;

--
-- TOC entry 223 (class 1259 OID 17186)
-- Name: files; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.files (
    id uuid DEFAULT public.uuid_v7() NOT NULL,
    mime_id uuid NOT NULL,
    datetime timestamp with time zone DEFAULT statement_timestamp() NOT NULL,
    notes character varying(1024) DEFAULT ''::character varying NOT NULL,
    created timestamp with time zone DEFAULT statement_timestamp() NOT NULL,
    creator_id uuid NOT NULL,
    is_private boolean DEFAULT true NOT NULL,
    orig_name character varying(256),
    metadata jsonb
);


ALTER TABLE public.files OWNER TO hiko;

--
-- TOC entry 3603 (class 0 OID 0)
-- Dependencies: 223
-- Name: COLUMN files.orig_name; Type: COMMENT; Schema: public; Owner: hiko
--

COMMENT ON COLUMN public.files.orig_name IS 'Original filename';


--
-- TOC entry 215 (class 1259 OID 16543)
-- Name: mime; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.mime (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(127) NOT NULL,
    extension character varying(16) NOT NULL
);


ALTER TABLE public.mime OWNER TO hiko;

--
-- TOC entry 230 (class 1259 OID 17670)
-- Name: v_files; Type: VIEW; Schema: public; Owner: hiko
--

CREATE VIEW public.v_files AS
 SELECT f.id,
    f.mime_id,
    m.name AS mime_name,
    m.extension,
    f.datetime,
    f.notes,
    f.created,
    f.creator_id,
    u.name AS creator_name,
    f.is_private
   FROM ((public.files f
     JOIN public.mime m ON ((f.mime_id = m.id)))
     JOIN public.users u ON ((f.creator_id = u.id)));


ALTER VIEW public.v_files OWNER TO hiko;

--
-- TOC entry 324 (class 1255 OID 17676)
-- Name: tfm_get_files_by_pool(uuid, uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_files_by_pool(s_id uuid, p_id uuid) RETURNS SETOF public.v_files
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
	u_is_admin boolean;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	SELECT u.is_admin FROM users u WHERE u.id=u_id INTO u_is_admin;
	PERFORM FROM pools p WHERE p.id=p_id AND (NOT p.is_private OR p.creator_id=u_id OR u_is_admin);
	IF NOT FOUND THEN RAISE 'Invalid pool id'; END IF;
	RETURN QUERY
	SELECT f.* FROM v_files f
	JOIN file_pool fp ON f.id=fp.file_id AND fp.pool_id=p_id
	WHERE NOT f.is_private OR f.creator_id=u_id OR u_is_admin;
END;
$$;


ALTER FUNCTION public.tfm_get_files_by_pool(s_id uuid, p_id uuid) OWNER TO hiko;

--
-- TOC entry 325 (class 1255 OID 17677)
-- Name: tfm_get_files_by_tag(uuid, uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_files_by_tag(s_id uuid, t_id uuid) RETURNS SETOF public.v_files
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	PERFORM FROM tags t WHERE t.id=t_id;
	IF NOT FOUND THEN RAISE 'Invalid tag id'; END IF;
	RETURN QUERY
	SELECT f.* FROM v_files f
	JOIN file_tag ft ON f.id=ft.file_id AND ft.tag_id=t_id
	WHERE NOT f.is_private OR f.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id);
END;
$$;


ALTER FUNCTION public.tfm_get_files_by_tag(s_id uuid, t_id uuid) OWNER TO hiko;

--
-- TOC entry 317 (class 1255 OID 17517)
-- Name: tfm_get_my_file_views(uuid, uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_my_file_views(s_id uuid, f_id uuid DEFAULT NULL::uuid) RETURNS TABLE(file_id uuid, datetime timestamp with time zone)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	RETURN QUERY
	SELECT fv.file_id, fv.datetime FROM file_views fv
	WHERE fv.user_id=u_id AND (f_id IS NULL OR fv.file_id=f_id);
END;
$$;


ALTER FUNCTION public.tfm_get_my_file_views(s_id uuid, f_id uuid) OWNER TO hiko;

--
-- TOC entry 214 (class 1259 OID 16534)
-- Name: sessions; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.sessions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    user_agent_id uuid NOT NULL,
    started timestamp with time zone DEFAULT statement_timestamp() NOT NULL,
    expires timestamp with time zone,
    last_seen timestamp with time zone
);


ALTER TABLE public.sessions OWNER TO hiko;

--
-- TOC entry 218 (class 1259 OID 16820)
-- Name: user_agents; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.user_agents (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(64) NOT NULL
);


ALTER TABLE public.user_agents OWNER TO hiko;

--
-- TOC entry 229 (class 1259 OID 17508)
-- Name: v_sessions; Type: VIEW; Schema: public; Owner: hiko
--

CREATE VIEW public.v_sessions AS
 SELECT s.id,
    s.user_id,
    u.name AS user_name,
    s.user_agent_id,
    ua.name AS user_agent_name,
    s.started,
    s.expires,
    s.last_seen
   FROM ((public.sessions s
     JOIN public.users u ON ((s.user_id = u.id)))
     JOIN public.user_agents ua ON ((s.user_agent_id = ua.id)));


ALTER VIEW public.v_sessions OWNER TO hiko;

--
-- TOC entry 300 (class 1255 OID 17512)
-- Name: tfm_get_my_sessions(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_my_sessions(s_id uuid) RETURNS SETOF public.v_sessions
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	RETURN QUERY
	SELECT s.* FROM v_sessions s
	WHERE s.user_id=u_id;
END;
$$;


ALTER FUNCTION public.tfm_get_my_sessions(s_id uuid) OWNER TO hiko;

--
-- TOC entry 221 (class 1259 OID 17125)
-- Name: tags; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.tags (
    id uuid DEFAULT public.uuid_v7() NOT NULL,
    name character varying(256) NOT NULL,
    notes character varying(1024) DEFAULT ''::character varying NOT NULL,
    color character(6),
    category_id uuid,
    created timestamp with time zone DEFAULT statement_timestamp() NOT NULL,
    creator_id uuid NOT NULL,
    is_private boolean DEFAULT true NOT NULL
);


ALTER TABLE public.tags OWNER TO hiko;

--
-- TOC entry 225 (class 1259 OID 17432)
-- Name: v_tags; Type: VIEW; Schema: public; Owner: hiko
--

CREATE VIEW public.v_tags AS
 SELECT t.id,
    t.name,
    t.notes,
    t.color,
    t.category_id,
    c.name AS category_name,
    c.color AS category_color,
    t.created,
    t.creator_id,
    u.name AS creator_name,
    t.is_private
   FROM ((public.tags t
     LEFT JOIN public.categories c ON ((t.category_id = c.id)))
     JOIN public.users u ON ((t.creator_id = u.id)));


ALTER VIEW public.v_tags OWNER TO hiko;

--
-- TOC entry 321 (class 1255 OID 17538)
-- Name: tfm_get_parent_tags(uuid, uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_parent_tags(s_id uuid, t_id uuid) RETURNS SETOF public.v_tags
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	PERFORM FROM tags t WHERE t.id=t_id;
	IF NOT FOUND THEN RAISE 'Invalid tag id'; END IF;
	RETURN QUERY
	SELECT t.* FROM v_tags t
	JOIN autotags a ON t.id=a.parent_id AND a.child_id=t_id
	WHERE NOT t.is_private OR t.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id);
END;
$$;


ALTER FUNCTION public.tfm_get_parent_tags(s_id uuid, t_id uuid) OWNER TO hiko;

--
-- TOC entry 224 (class 1259 OID 17314)
-- Name: pools; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.pools (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(256) NOT NULL,
    notes character varying(1024) DEFAULT ''::character varying NOT NULL,
    created timestamp with time zone DEFAULT statement_timestamp() NOT NULL,
    parent_id uuid,
    creator_id uuid NOT NULL,
    is_private boolean DEFAULT true NOT NULL
);


ALTER TABLE public.pools OWNER TO hiko;

--
-- TOC entry 227 (class 1259 OID 17445)
-- Name: v_pools; Type: VIEW; Schema: public; Owner: hiko
--

CREATE VIEW public.v_pools AS
 SELECT p.id,
    p.name,
    p.notes,
    p.created,
    p.parent_id,
    pp.name AS parent_name,
    p.creator_id,
    u.name AS creator_name,
    p.is_private
   FROM ((public.pools p
     LEFT JOIN public.pools pp ON ((p.parent_id = pp.id)))
     JOIN public.users u ON ((p.creator_id = u.id)));


ALTER VIEW public.v_pools OWNER TO hiko;

--
-- TOC entry 303 (class 1255 OID 17514)
-- Name: tfm_get_pools(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_pools(s_id uuid) RETURNS SETOF public.v_pools
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	RETURN QUERY
	SELECT p.* FROM v_pools p
	WHERE NOT p.is_private OR p.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id);
END;
$$;


ALTER FUNCTION public.tfm_get_pools(s_id uuid) OWNER TO hiko;

--
-- TOC entry 304 (class 1255 OID 17515)
-- Name: tfm_get_tags(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_tags(s_id uuid) RETURNS SETOF public.v_tags
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	RETURN QUERY
	SELECT t.* FROM v_tags t
	WHERE NOT t.is_private OR t.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id);
END;
$$;


ALTER FUNCTION public.tfm_get_tags(s_id uuid) OWNER TO hiko;

--
-- TOC entry 316 (class 1255 OID 17516)
-- Name: tfm_get_tags_by_file(uuid, uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_get_tags_by_file(s_id uuid, f_id uuid) RETURNS SETOF public.v_tags
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	PERFORM FROM files f WHERE f.id=f_id;
	IF NOT FOUND THEN RAISE 'Invalid file id'; END IF;
	RETURN QUERY
	SELECT t.* FROM v_tags t
	JOIN file_tag ft ON t.id=ft.tag_id AND ft.file_id=f_id
	WHERE NOT t.is_private OR t.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id);
END;
$$;


ALTER FUNCTION public.tfm_get_tags_by_file(s_id uuid, f_id uuid) OWNER TO hiko;

--
-- TOC entry 294 (class 1255 OID 17484)
-- Name: tfm_remove_autotag(uuid, uuid, uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_remove_autotag(IN s_id uuid, IN tc_id uuid, IN tp_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	DELETE FROM autotags a WHERE a.child_id=tc_id AND a.parent_id=tp_id;
END;
$$;


ALTER PROCEDURE public.tfm_remove_autotag(IN s_id uuid, IN tc_id uuid, IN tp_id uuid) OWNER TO hiko;

--
-- TOC entry 296 (class 1255 OID 17481)
-- Name: tfm_remove_category(uuid, uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_remove_category(IN s_id uuid, IN c_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT c.is_private OR c.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM categories c WHERE c.id=c_id)
	THEN RAISE 'Not allowed'; END IF;
	DELETE FROM categories c WHERE c.id=c_id;
END;
$$;


ALTER PROCEDURE public.tfm_remove_category(IN s_id uuid, IN c_id uuid) OWNER TO hiko;

--
-- TOC entry 310 (class 1255 OID 17479)
-- Name: tfm_remove_file(uuid, uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_remove_file(IN s_id uuid, IN f_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT f.is_private OR f.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM files f WHERE f.id=f_id)
	THEN RAISE 'Not allowed'; END IF;
	DELETE FROM files f WHERE f.id=f_id;
END;
$$;


ALTER PROCEDURE public.tfm_remove_file(IN s_id uuid, IN f_id uuid) OWNER TO hiko;

--
-- TOC entry 308 (class 1255 OID 17485)
-- Name: tfm_remove_file_to_pool(uuid, uuid, uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_remove_file_to_pool(IN s_id uuid, IN f_id uuid, IN p_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT p.is_private OR p.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM pools p WHERE p.id=p_id)
	THEN RAISE 'Not allowed'; END IF;
	DELETE FROM file_pool fp WHERE fp.file_id=f_id AND fp.pool_id=p_id;
END;
$$;


ALTER PROCEDURE public.tfm_remove_file_to_pool(IN s_id uuid, IN f_id uuid, IN p_id uuid) OWNER TO hiko;

--
-- TOC entry 293 (class 1255 OID 17483)
-- Name: tfm_remove_file_to_tag(uuid, uuid, uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_remove_file_to_tag(IN s_id uuid, IN f_id uuid, IN t_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id) THEN RAISE 'Not allowed'; END IF;
	DELETE FROM file_tag ft WHERE ft.file_id=f_id AND ft.tag_id=t_id;
END;
$$;


ALTER PROCEDURE public.tfm_remove_file_to_tag(IN s_id uuid, IN f_id uuid, IN t_id uuid) OWNER TO hiko;

--
-- TOC entry 297 (class 1255 OID 17482)
-- Name: tfm_remove_pool(uuid, uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_remove_pool(IN s_id uuid, IN p_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT p.is_private OR p.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM pools p WHERE p.id=p_id)
	THEN RAISE 'Not allowed'; END IF;
	DELETE FROM pools p WHERE p.id=p_id;
END;
$$;


ALTER PROCEDURE public.tfm_remove_pool(IN s_id uuid, IN p_id uuid) OWNER TO hiko;

--
-- TOC entry 295 (class 1255 OID 17480)
-- Name: tfm_remove_tag(uuid, uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_remove_tag(IN s_id uuid, IN t_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	IF
		NOT (SELECT u.can_edit FROM users u WHERE u.id=u_id)
		OR
		NOT (SELECT (NOT t.is_private OR t.creator_id=u_id OR (SELECT u.is_admin FROM users u WHERE u.id=u_id)) FROM tags t WHERE t.id=t_id)
	THEN RAISE 'Not allowed'; END IF;
	DELETE FROM tags t WHERE t.id=t_id;
END;
$$;


ALTER PROCEDURE public.tfm_remove_tag(IN s_id uuid, IN t_id uuid) OWNER TO hiko;

--
-- TOC entry 291 (class 1255 OID 16848)
-- Name: tfm_session_request(uuid, text); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_session_request(u_id uuid, u_agent text, OUT s_id uuid) RETURNS uuid
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	ua_id UUID;
BEGIN
	PERFORM FROM users WHERE id=u_id;
	IF NOT FOUND THEN RAISE 'User not exists'; END IF;
	SELECT id FROM user_agents WHERE name=u_agent INTO ua_id;
	IF NOT FOUND THEN RAISE 'Unsupported user agent'; END IF;
	INSERT INTO sessions(user_id, user_agent_id) VALUES(u_id, ua_id) RETURNING id INTO s_id;
END;
$$;


ALTER FUNCTION public.tfm_session_request(u_id uuid, u_agent text, OUT s_id uuid) OWNER TO hiko;

--
-- TOC entry 313 (class 1255 OID 17503)
-- Name: tfm_session_terminate(uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_session_terminate(IN s_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE
	u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	DELETE FROM sessions s WHERE s.id=s_id;
END;
$$;


ALTER PROCEDURE public.tfm_session_terminate(IN s_id uuid) OWNER TO hiko;

--
-- TOC entry 314 (class 1255 OID 17504)
-- Name: tfm_session_username(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_session_username(s_id uuid) RETURNS character varying
    LANGUAGE sql SECURITY DEFINER
    AS $$
SELECT u.name FROM users u JOIN sessions s ON s.user_id=u.id;
$$;


ALTER FUNCTION public.tfm_session_username(s_id uuid) OWNER TO hiko;

--
-- TOC entry 326 (class 1255 OID 26730)
-- Name: tfm_session_validate(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_session_validate(s_id uuid) RETURNS uuid
    LANGUAGE sql SECURITY DEFINER PARALLEL SAFE
    AS $$
UPDATE sessions SET last_seen=statement_timestamp() WHERE id=s_id RETURNING user_id;
$$;


ALTER FUNCTION public.tfm_session_validate(s_id uuid) OWNER TO hiko;

--
-- TOC entry 292 (class 1255 OID 16801)
-- Name: tfm_user_auth(character varying, text); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_user_auth(u_name character varying, u_password text) RETURNS uuid
    LANGUAGE plpgsql STABLE SECURITY DEFINER
    AS $$
DECLARE
	selected_user users%ROWTYPE;
BEGIN
SELECT * FROM users
WHERE users.name=u_name
INTO selected_user;
IF selected_user.password=crypt(u_password, selected_user.password) THEN
	RETURN selected_user.id;
END IF;
RAISE 'Authorization failed';
END;
$$;


ALTER FUNCTION public.tfm_user_auth(u_name character varying, u_password text) OWNER TO hiko;

--
-- TOC entry 3615 (class 0 OID 0)
-- Dependencies: 292
-- Name: FUNCTION tfm_user_auth(u_name character varying, u_password text); Type: COMMENT; Schema: public; Owner: hiko
--

COMMENT ON FUNCTION public.tfm_user_auth(u_name character varying, u_password text) IS 'Returns user UUID if username and password are valid otherwise nil UUID.';


--
-- TOC entry 306 (class 1255 OID 17451)
-- Name: tfm_user_create(text, text, boolean, boolean); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_user_create(u_name text, u_password text, u_is_admin boolean DEFAULT false, u_can_edit boolean DEFAULT false, OUT u_id uuid) RETURNS uuid
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
BEGIN
	INSERT INTO users(name, password, is_admin, can_edit) VALUES(u_name, crypt(u_password, gen_salt('bf')), u_is_admin, u_can_edit) RETURNING id INTO u_id;
END;
$$;


ALTER FUNCTION public.tfm_user_create(u_name text, u_password text, u_is_admin boolean, u_can_edit boolean, OUT u_id uuid) OWNER TO hiko;

--
-- TOC entry 327 (class 1255 OID 27477)
-- Name: tfm_user_get_info(uuid); Type: FUNCTION; Schema: public; Owner: hiko
--

CREATE FUNCTION public.tfm_user_get_info(s_id uuid, OUT user_info public.users) RETURNS public.users
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	SELECT * FROM users WHERE id=u_id INTO user_info;
END;
$$;


ALTER FUNCTION public.tfm_user_get_info(s_id uuid, OUT user_info public.users) OWNER TO hiko;

--
-- TOC entry 302 (class 1255 OID 17420)
-- Name: tfm_view_file(uuid, uuid); Type: PROCEDURE; Schema: public; Owner: hiko
--

CREATE PROCEDURE public.tfm_view_file(IN s_id uuid, IN f_id uuid)
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
DECLARE u_id uuid;
BEGIN
	SELECT tfm_session_validate(s_id) INTO u_id;
	IF u_id IS NULL THEN RAISE 'Invalid session id'; END IF;
	PERFORM FROM files f WHERE f.id=f_id AND (NOT f.is_private OR f.creator_id=u_id);
	IF NOT FOUND THEN RAISE 'Invalid file id'; END IF;
	INSERT INTO file_views(file_id, user_id) VALUES(f_id, u_id);
END;
$$;


ALTER PROCEDURE public.tfm_view_file(IN s_id uuid, IN f_id uuid) OWNER TO hiko;

--
-- TOC entry 232 (class 1259 OID 34878)
-- Name: acl; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.acl (
    user_id uuid NOT NULL,
    object_id uuid NOT NULL,
    read boolean DEFAULT true NOT NULL,
    write boolean DEFAULT false NOT NULL
);


ALTER TABLE public.acl OWNER TO hiko;

--
-- TOC entry 217 (class 1259 OID 16610)
-- Name: file_pool; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.file_pool (
    file_id uuid NOT NULL,
    pool_id uuid NOT NULL
);


ALTER TABLE public.file_pool OWNER TO hiko;

--
-- TOC entry 216 (class 1259 OID 16605)
-- Name: file_tag; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.file_tag (
    file_id uuid NOT NULL,
    tag_id uuid NOT NULL
);


ALTER TABLE public.file_tag OWNER TO hiko;

--
-- TOC entry 219 (class 1259 OID 17032)
-- Name: file_views; Type: TABLE; Schema: public; Owner: hiko
--

CREATE TABLE public.file_views (
    file_id uuid NOT NULL,
    datetime timestamp with time zone DEFAULT statement_timestamp() NOT NULL,
    user_id uuid NOT NULL
);


ALTER TABLE public.file_views OWNER TO hiko;

--
-- TOC entry 3366 (class 2606 OID 17357)
-- Name: categories chk__categories__color__hex; Type: CHECK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE public.categories
    ADD CONSTRAINT chk__categories__color__hex CHECK (((color IS NULL) OR (color ~* '^[A-Fa-f0-9]{6}$'::text))) NOT VALID;


--
-- TOC entry 3621 (class 0 OID 0)
-- Dependencies: 3366
-- Name: CONSTRAINT chk__categories__color__hex ON categories; Type: COMMENT; Schema: public; Owner: hiko
--

COMMENT ON CONSTRAINT chk__categories__color__hex ON public.categories IS 'Check if `color` is a valid HEX color';


--
-- TOC entry 3365 (class 2606 OID 17356)
-- Name: tags chk__tags___color__hex; Type: CHECK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE public.tags
    ADD CONSTRAINT chk__tags___color__hex CHECK (((color IS NULL) OR (color ~* '^[A-Fa-f0-9]{6}$'::text))) NOT VALID;


--
-- TOC entry 3622 (class 0 OID 0)
-- Dependencies: 3365
-- Name: CONSTRAINT chk__tags___color__hex ON tags; Type: COMMENT; Schema: public; Owner: hiko
--

COMMENT ON CONSTRAINT chk__tags___color__hex ON public.tags IS 'Check if `color` is a valid HEX color';


--
-- TOC entry 3426 (class 2606 OID 34884)
-- Name: acl prm__acl; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.acl
    ADD CONSTRAINT prm__acl PRIMARY KEY (user_id, object_id);


--
-- TOC entry 3399 (class 2606 OID 17070)
-- Name: autotags prm__autotags; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.autotags
    ADD CONSTRAINT prm__autotags PRIMARY KEY (child_id, parent_id);


--
-- TOC entry 3410 (class 2606 OID 17173)
-- Name: categories prm__categories; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT prm__categories PRIMARY KEY (id);


--
-- TOC entry 3386 (class 2606 OID 16614)
-- Name: file_pool prm__file_pool; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_pool
    ADD CONSTRAINT prm__file_pool PRIMARY KEY (file_id, pool_id);


--
-- TOC entry 3382 (class 2606 OID 16609)
-- Name: file_tag prm__file_tag; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_tag
    ADD CONSTRAINT prm__file_tag PRIMARY KEY (file_id, tag_id);


--
-- TOC entry 3395 (class 2606 OID 17037)
-- Name: file_views prm__file_views; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_views
    ADD CONSTRAINT prm__file_views PRIMARY KEY (file_id, datetime, user_id);


--
-- TOC entry 3418 (class 2606 OID 17196)
-- Name: files prm__files; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.files
    ADD CONSTRAINT prm__files PRIMARY KEY (id);


--
-- TOC entry 3376 (class 2606 OID 16548)
-- Name: mime prm__mime; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.mime
    ADD CONSTRAINT prm__mime PRIMARY KEY (id);


--
-- TOC entry 3422 (class 2606 OID 17323)
-- Name: pools prm__pools; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.pools
    ADD CONSTRAINT prm__pools PRIMARY KEY (id);


--
-- TOC entry 3374 (class 2606 OID 16542)
-- Name: sessions prm__sessions; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT prm__sessions PRIMARY KEY (id);


--
-- TOC entry 3404 (class 2606 OID 17135)
-- Name: tags prm__tags; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT prm__tags PRIMARY KEY (id);


--
-- TOC entry 3388 (class 2606 OID 16824)
-- Name: user_agents prm__user_agents; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.user_agents
    ADD CONSTRAINT prm__user_agents PRIMARY KEY (id);


--
-- TOC entry 3368 (class 2606 OID 16531)
-- Name: users prm__users; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT prm__users PRIMARY KEY (id);


--
-- TOC entry 3412 (class 2606 OID 17175)
-- Name: categories uni__categories__name; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT uni__categories__name UNIQUE (name);


--
-- TOC entry 3378 (class 2606 OID 16550)
-- Name: mime uni__mime__name; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.mime
    ADD CONSTRAINT uni__mime__name UNIQUE (name);


--
-- TOC entry 3424 (class 2606 OID 17325)
-- Name: pools uni__pools__name; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.pools
    ADD CONSTRAINT uni__pools__name UNIQUE (name);


--
-- TOC entry 3406 (class 2606 OID 17137)
-- Name: tags uni__tags__name; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT uni__tags__name UNIQUE (name);


--
-- TOC entry 3390 (class 2606 OID 16826)
-- Name: user_agents uni__user_agents__name; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.user_agents
    ADD CONSTRAINT uni__user_agents__name UNIQUE (name);


--
-- TOC entry 3370 (class 2606 OID 16533)
-- Name: users uni__users__name; Type: CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT uni__users__name UNIQUE (name);


--
-- TOC entry 3396 (class 1259 OID 27375)
-- Name: idx__autotags__child_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__autotags__child_id ON public.autotags USING hash (child_id);


--
-- TOC entry 3397 (class 1259 OID 27376)
-- Name: idx__autotags__parent_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__autotags__parent_id ON public.autotags USING hash (parent_id);


--
-- TOC entry 3407 (class 1259 OID 27389)
-- Name: idx__categories__created; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__categories__created ON public.categories USING btree (created DESC NULLS LAST);


--
-- TOC entry 3408 (class 1259 OID 27390)
-- Name: idx__categories__creator_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__categories__creator_id ON public.categories USING hash (creator_id);


--
-- TOC entry 3383 (class 1259 OID 27379)
-- Name: idx__file_pool__file_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__file_pool__file_id ON public.file_pool USING hash (file_id);


--
-- TOC entry 3384 (class 1259 OID 27380)
-- Name: idx__file_pool__pool_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__file_pool__pool_id ON public.file_pool USING hash (pool_id);


--
-- TOC entry 3379 (class 1259 OID 27377)
-- Name: idx__file_tag__file_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__file_tag__file_id ON public.file_tag USING hash (file_id);


--
-- TOC entry 3380 (class 1259 OID 27378)
-- Name: idx__file_tag__tag_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__file_tag__tag_id ON public.file_tag USING hash (tag_id);


--
-- TOC entry 3391 (class 1259 OID 27382)
-- Name: idx__file_views__datetime; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__file_views__datetime ON public.file_views USING btree (datetime DESC NULLS LAST);


--
-- TOC entry 3392 (class 1259 OID 27381)
-- Name: idx__file_views__file_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__file_views__file_id ON public.file_views USING hash (file_id);


--
-- TOC entry 3393 (class 1259 OID 27391)
-- Name: idx__file_views__user_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__file_views__user_id ON public.file_views USING hash (user_id);


--
-- TOC entry 3413 (class 1259 OID 27385)
-- Name: idx__files__created; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__files__created ON public.files USING btree (created DESC NULLS LAST);


--
-- TOC entry 3414 (class 1259 OID 27392)
-- Name: idx__files__creator_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__files__creator_id ON public.files USING hash (creator_id);


--
-- TOC entry 3415 (class 1259 OID 27384)
-- Name: idx__files__datetime; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__files__datetime ON public.files USING btree (datetime DESC NULLS LAST);


--
-- TOC entry 3416 (class 1259 OID 27383)
-- Name: idx__files__mime_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__files__mime_id ON public.files USING hash (mime_id);


--
-- TOC entry 3419 (class 1259 OID 27386)
-- Name: idx__pools__created; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__pools__created ON public.pools USING btree (created DESC NULLS LAST);


--
-- TOC entry 3420 (class 1259 OID 27393)
-- Name: idx__pools__creator_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__pools__creator_id ON public.pools USING hash (creator_id);


--
-- TOC entry 3371 (class 1259 OID 63917)
-- Name: idx__sessions__user_agent_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__sessions__user_agent_id ON public.sessions USING hash (user_agent_id);


--
-- TOC entry 3372 (class 1259 OID 63916)
-- Name: idx__sessions__user_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__sessions__user_id ON public.sessions USING hash (user_id);


--
-- TOC entry 3400 (class 1259 OID 27388)
-- Name: idx__tags__category_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__tags__category_id ON public.tags USING hash (category_id);


--
-- TOC entry 3401 (class 1259 OID 27387)
-- Name: idx__tags__created; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__tags__created ON public.tags USING btree (created DESC NULLS LAST);


--
-- TOC entry 3402 (class 1259 OID 63915)
-- Name: idx__tags__creator_id; Type: INDEX; Schema: public; Owner: hiko
--

CREATE INDEX idx__tags__creator_id ON public.tags USING hash (creator_id);


--
-- TOC entry 3444 (class 2606 OID 34885)
-- Name: acl frn__acl__user_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.acl
    ADD CONSTRAINT frn__acl__user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3435 (class 2606 OID 17159)
-- Name: autotags frn__autotags__child_id_; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.autotags
    ADD CONSTRAINT frn__autotags__child_id_ FOREIGN KEY (child_id) REFERENCES public.tags(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3436 (class 2606 OID 17154)
-- Name: autotags frn__autotags__parent_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.autotags
    ADD CONSTRAINT frn__autotags__parent_id FOREIGN KEY (parent_id) REFERENCES public.tags(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3439 (class 2606 OID 17176)
-- Name: categories frn__categories__creator_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT frn__categories__creator_id FOREIGN KEY (creator_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE RESTRICT;


--
-- TOC entry 3431 (class 2606 OID 17222)
-- Name: file_pool frn__file_pool__file_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_pool
    ADD CONSTRAINT frn__file_pool__file_id FOREIGN KEY (file_id) REFERENCES public.files(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3432 (class 2606 OID 17336)
-- Name: file_pool frn__file_pool__pool_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_pool
    ADD CONSTRAINT frn__file_pool__pool_id FOREIGN KEY (pool_id) REFERENCES public.pools(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3429 (class 2606 OID 17212)
-- Name: file_tag frn__file_tag__file_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_tag
    ADD CONSTRAINT frn__file_tag__file_id FOREIGN KEY (file_id) REFERENCES public.files(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3430 (class 2606 OID 17149)
-- Name: file_tag frn__file_tag__tag_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_tag
    ADD CONSTRAINT frn__file_tag__tag_id FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3433 (class 2606 OID 17217)
-- Name: file_views frn__file_views__file_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_views
    ADD CONSTRAINT frn__file_views__file_id FOREIGN KEY (file_id) REFERENCES public.files(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3434 (class 2606 OID 17043)
-- Name: file_views frn__file_views__user_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.file_views
    ADD CONSTRAINT frn__file_views__user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3440 (class 2606 OID 17389)
-- Name: files frn__files__creator_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.files
    ADD CONSTRAINT frn__files__creator_id FOREIGN KEY (creator_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE RESTRICT;


--
-- TOC entry 3441 (class 2606 OID 17202)
-- Name: files frn__files__mime_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.files
    ADD CONSTRAINT frn__files__mime_id FOREIGN KEY (mime_id) REFERENCES public.mime(id) ON UPDATE CASCADE ON DELETE RESTRICT;


--
-- TOC entry 3442 (class 2606 OID 17326)
-- Name: pools frn__pools__creator_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.pools
    ADD CONSTRAINT frn__pools__creator_id FOREIGN KEY (creator_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE RESTRICT;


--
-- TOC entry 3443 (class 2606 OID 17331)
-- Name: pools frn__pools__parent_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.pools
    ADD CONSTRAINT frn__pools__parent_id FOREIGN KEY (parent_id) REFERENCES public.pools(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- TOC entry 3427 (class 2606 OID 17001)
-- Name: sessions frn__sessions__user_agent_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT frn__sessions__user_agent_id FOREIGN KEY (user_agent_id) REFERENCES public.user_agents(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3428 (class 2606 OID 17006)
-- Name: sessions frn__sessions__user_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT frn__sessions__user_id FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- TOC entry 3437 (class 2606 OID 17181)
-- Name: tags frn__tags__category_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT frn__tags__category_id FOREIGN KEY (category_id) REFERENCES public.categories(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- TOC entry 3438 (class 2606 OID 17143)
-- Name: tags frn__tags__creator_id; Type: FK CONSTRAINT; Schema: public; Owner: hiko
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT frn__tags__creator_id FOREIGN KEY (creator_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE RESTRICT;


--
-- TOC entry 3595 (class 0 OID 0)
-- Dependencies: 7
-- Name: SCHEMA public; Type: ACL; Schema: -; Owner: hiko
--

REVOKE USAGE ON SCHEMA public FROM PUBLIC;
GRANT ALL ON SCHEMA public TO PUBLIC;


--
-- TOC entry 3598 (class 0 OID 0)
-- Dependencies: 220
-- Name: TABLE autotags; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.autotags FROM hiko;
GRANT SELECT ON TABLE public.autotags TO grafana;


--
-- TOC entry 3599 (class 0 OID 0)
-- Dependencies: 228
-- Name: TABLE v_autotags; Type: ACL; Schema: public; Owner: hiko
--

GRANT SELECT ON TABLE public.v_autotags TO grafana;


--
-- TOC entry 3600 (class 0 OID 0)
-- Dependencies: 222
-- Name: TABLE categories; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.categories FROM hiko;
GRANT SELECT ON TABLE public.categories TO grafana;


--
-- TOC entry 3601 (class 0 OID 0)
-- Dependencies: 213
-- Name: TABLE users; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.users FROM hiko;
GRANT SELECT ON TABLE public.users TO grafana;


--
-- TOC entry 3602 (class 0 OID 0)
-- Dependencies: 226
-- Name: TABLE v_categories; Type: ACL; Schema: public; Owner: hiko
--

GRANT SELECT ON TABLE public.v_categories TO grafana;


--
-- TOC entry 3604 (class 0 OID 0)
-- Dependencies: 223
-- Name: TABLE files; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.files FROM hiko;
GRANT SELECT ON TABLE public.files TO grafana;


--
-- TOC entry 3605 (class 0 OID 0)
-- Dependencies: 215
-- Name: TABLE mime; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.mime FROM hiko;
GRANT SELECT ON TABLE public.mime TO tanabata;
GRANT SELECT ON TABLE public.mime TO grafana;


--
-- TOC entry 3606 (class 0 OID 0)
-- Dependencies: 230
-- Name: TABLE v_files; Type: ACL; Schema: public; Owner: hiko
--

GRANT SELECT ON TABLE public.v_files TO grafana;


--
-- TOC entry 3607 (class 0 OID 0)
-- Dependencies: 214
-- Name: TABLE sessions; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.sessions FROM hiko;
GRANT SELECT ON TABLE public.sessions TO grafana;


--
-- TOC entry 3608 (class 0 OID 0)
-- Dependencies: 218
-- Name: TABLE user_agents; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.user_agents FROM hiko;
GRANT SELECT ON TABLE public.user_agents TO tanabata;
GRANT SELECT ON TABLE public.user_agents TO grafana;


--
-- TOC entry 3609 (class 0 OID 0)
-- Dependencies: 229
-- Name: TABLE v_sessions; Type: ACL; Schema: public; Owner: hiko
--

GRANT SELECT ON TABLE public.v_sessions TO grafana;


--
-- TOC entry 3610 (class 0 OID 0)
-- Dependencies: 221
-- Name: TABLE tags; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.tags FROM hiko;
GRANT SELECT ON TABLE public.tags TO grafana;


--
-- TOC entry 3611 (class 0 OID 0)
-- Dependencies: 225
-- Name: TABLE v_tags; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.v_tags FROM hiko;
GRANT SELECT ON TABLE public.v_tags TO grafana;


--
-- TOC entry 3612 (class 0 OID 0)
-- Dependencies: 224
-- Name: TABLE pools; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.pools FROM hiko;
GRANT SELECT ON TABLE public.pools TO grafana;


--
-- TOC entry 3613 (class 0 OID 0)
-- Dependencies: 227
-- Name: TABLE v_pools; Type: ACL; Schema: public; Owner: hiko
--

GRANT SELECT ON TABLE public.v_pools TO grafana;


--
-- TOC entry 3614 (class 0 OID 0)
-- Dependencies: 291
-- Name: FUNCTION tfm_session_request(u_id uuid, u_agent text, OUT s_id uuid); Type: ACL; Schema: public; Owner: hiko
--

GRANT ALL ON FUNCTION public.tfm_session_request(u_id uuid, u_agent text, OUT s_id uuid) TO tanabata;


--
-- TOC entry 3616 (class 0 OID 0)
-- Dependencies: 292
-- Name: FUNCTION tfm_user_auth(u_name character varying, u_password text); Type: ACL; Schema: public; Owner: hiko
--

GRANT ALL ON FUNCTION public.tfm_user_auth(u_name character varying, u_password text) TO tanabata;


--
-- TOC entry 3617 (class 0 OID 0)
-- Dependencies: 232
-- Name: TABLE acl; Type: ACL; Schema: public; Owner: hiko
--

GRANT SELECT ON TABLE public.acl TO grafana;


--
-- TOC entry 3618 (class 0 OID 0)
-- Dependencies: 217
-- Name: TABLE file_pool; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.file_pool FROM hiko;
GRANT SELECT ON TABLE public.file_pool TO grafana;


--
-- TOC entry 3619 (class 0 OID 0)
-- Dependencies: 216
-- Name: TABLE file_tag; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.file_tag FROM hiko;
GRANT SELECT ON TABLE public.file_tag TO grafana;


--
-- TOC entry 3620 (class 0 OID 0)
-- Dependencies: 219
-- Name: TABLE file_views; Type: ACL; Schema: public; Owner: hiko
--

REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLE public.file_views FROM hiko;
GRANT SELECT ON TABLE public.file_views TO tanabata;
GRANT SELECT ON TABLE public.file_views TO grafana;


--
-- TOC entry 2192 (class 826 OID 61487)
-- Name: DEFAULT PRIVILEGES FOR TABLES; Type: DEFAULT ACL; Schema: -; Owner: hiko
--

ALTER DEFAULT PRIVILEGES FOR ROLE hiko REVOKE SELECT,INSERT,REFERENCES,DELETE,TRIGGER,TRUNCATE,UPDATE ON TABLES FROM hiko;


-- Completed on 2026-03-31 00:31:49

--
-- PostgreSQL database dump complete
--

