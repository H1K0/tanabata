from configparser import ConfigParser
from psycopg2.pool import ThreadedConnectionPool
from psycopg2.extras import RealDictCursor
from contextlib import contextmanager
from os import access, W_OK, makedirs, chmod, system
from os.path import isfile, join, basename
from shutil import move
from magic import Magic
from preview_generator.manager import PreviewManager

conf = None

mage = None
previewer = None

db_pool = None

DEFAULT_SORTING = {
	"files": {
		"key": "created",
		"asc": False
	},
	"tags": {
		"key": "created",
		"asc": False
	},
	"categories": {
		"key": "created",
		"asc": False
	},
	"pools": {
		"key": "created",
		"asc": False
	},
}


def Initialize(conf_path="/etc/tfm/tfm.conf"):
	global mage, previewer
	load_config(conf_path)
	mage = Magic(mime=True)
	previewer = PreviewManager(conf["Paths"]["Thumbs"])
	db_connect(conf["DB.limits"]["MinimumConnections"], conf["DB.limits"]["MaximumConnections"], **conf["DB.params"])


def load_config(path):
	global conf
	conf = ConfigParser()
	conf.read(path)


def db_connect(minconn, maxconn, **kwargs):
	global db_pool
	db_pool = ThreadedConnectionPool(minconn, maxconn, **kwargs)


@contextmanager
def _db_cursor():
	global db_pool
	try:
		conn = db_pool.getconn()
	except:
		raise RuntimeError("Database not connected")
	try:
		with conn.cursor(cursor_factory=RealDictCursor) as cur:
			yield cur
			conn.commit()
	except:
		conn.rollback()
		raise
	finally:
		db_pool.putconn(conn)


def _validate_column_name(cur, table, column):
	cur.execute("SELECT get_column_names(%s) AS name", (table,))
	if all([column!=col["name"] for col in cur.fetchall()]):
		raise RuntimeError("Invalid column name")


def authorize(username, password, useragent):
	with _db_cursor() as cur:
		cur.execute("SELECT tfm_session_request(tfm_user_auth(%s, %s), %s) AS sid", (username, password, useragent))
		sid = cur.fetchone()["sid"]
	return TSession(sid)


class TSession:
	sid = None

	def __init__(self, sid):
		with _db_cursor() as cur:
			cur.execute("SELECT tfm_session_validate(%s) IS NOT NULL AS valid", (sid,))
			if not cur.fetchone()["valid"]:
				raise RuntimeError("Invalid sid")
			self.sid = sid

	def terminate(self):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_session_terminate(%s)", (self.sid,))
		del self

	@property
	def username(self):
		with _db_cursor() as cur:
			cur.execute("SELECT tfm_session_username(%s) AS name", (self.sid,))
			return cur.fetchone()["name"]
	
	@property
	def is_admin(self):
		with _db_cursor() as cur:
			cur.execute("SELECT * FROM tfm_user_get_info(%s)", (self.sid,))
			return cur.fetchone()["can_edit"]

	def get_files(self, order_key=DEFAULT_SORTING["files"]["key"], order_asc=DEFAULT_SORTING["files"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_files", order_key)
			cur.execute("SELECT * FROM tfm_get_files(%%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid,))
			return list(map(dict, cur.fetchall()))

	def get_files_by_filter(self, philter=None, order_key=DEFAULT_SORTING["files"]["key"], order_asc=DEFAULT_SORTING["files"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_files", order_key)
			cur.execute("SELECT * FROM tfm_get_files_by_filter(%%s, %%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid, philter))
			return list(map(dict, cur.fetchall()))

	def get_tags(self, order_key=DEFAULT_SORTING["tags"]["key"], order_asc=DEFAULT_SORTING["tags"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_tags", order_key)
			cur.execute("SELECT * FROM tfm_get_tags(%%s) ORDER BY %s %s, name ASC OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid,))
			return list(map(dict, cur.fetchall()))

	def get_categories(self, order_key=DEFAULT_SORTING["categories"]["key"], order_asc=DEFAULT_SORTING["categories"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_categories", order_key)
			cur.execute("SELECT * FROM tfm_get_categories(%%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid,))
			return list(map(dict, cur.fetchall()))

	def get_pools(self, order_key=DEFAULT_SORTING["pools"]["key"], order_asc=DEFAULT_SORTING["pools"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_pools", order_key)
			cur.execute("SELECT * FROM tfm_get_pools(%%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid,))
			return list(map(dict, cur.fetchall()))

	def get_autotags(self, order_key="child_id", order_asc=True, offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_autotags", order_key)
			cur.execute("SELECT * FROM tfm_get_autotags(%%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid,))
			return list(map(dict, cur.fetchall()))

	def get_my_sessions(self, order_key="started", order_asc=False, offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_sessions", order_key)
			cur.execute("SELECT * FROM tfm_get_my_sessions(%%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid,))
			return list(map(dict, cur.fetchall()))

	def get_tags_by_file(self, file_id, order_key=DEFAULT_SORTING["tags"]["key"], order_asc=DEFAULT_SORTING["tags"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_tags", order_key)
			cur.execute("SELECT * FROM tfm_get_tags_by_file(%%s, %%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid, file_id))
			return list(map(dict, cur.fetchall()))

	def get_files_by_tag(self, tag_id, order_key=DEFAULT_SORTING["files"]["key"], order_asc=DEFAULT_SORTING["files"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_files", order_key)
			cur.execute("SELECT * FROM tfm_get_files_by_tag(%%s, %%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid, tag_id))
			return list(map(dict, cur.fetchall()))

	def get_files_by_pool(self, pool_id, order_key=DEFAULT_SORTING["files"]["key"], order_asc=DEFAULT_SORTING["files"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_files", order_key)
			cur.execute("SELECT * FROM tfm_get_files_by_pool(%%s, %%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid, pool_id))
			return list(map(dict, cur.fetchall()))

	def get_parent_tags(self, tag_id, order_key=DEFAULT_SORTING["tags"]["key"], order_asc=DEFAULT_SORTING["tags"]["asc"], offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_tags", order_key)
			cur.execute("SELECT * FROM tfm_get_parent_tags(%%s, %%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid, tag_id))
			return list(map(dict, cur.fetchall()))

	def get_my_file_views(self, file_id=None, order_key="datetime", order_asc=False, offset=0, limit=None):
		with _db_cursor() as cur:
			_validate_column_name(cur, "v_files", order_key)
			cur.execute("SELECT * FROM tfm_get_my_file_views(%%s, %%s) ORDER BY %s %s OFFSET %s LIMIT %s" % (
				order_key,
				"ASC" if order_asc else "DESC",
				int(offset),
				int(limit) if limit is not None else "ALL"
			), (self.sid, file_id))
			return list(map(dict, cur.fetchall()))

	def get_file(self, file_id):
		with _db_cursor() as cur:
			cur.execute("SELECT * FROM tfm_get_files(%s) WHERE id=%s", (self.sid, file_id))
			return cur.fetchone()

	def get_tag(self, tag_id):
		with _db_cursor() as cur:
			cur.execute("SELECT * FROM tfm_get_tags(%s) WHERE id=%s", (self.sid, tag_id))
			return cur.fetchone()

	def get_category(self, category_id):
		with _db_cursor() as cur:
			cur.execute("SELECT * FROM tfm_get_categories(%s) WHERE id=%s", (self.sid, category_id))
			return cur.fetchone()

	def view_file(self, file_id):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_view_file(%s, %s)", (self.sid, file_id))

	def add_file(self, path, datetime=None, notes=None, is_private=None, orig_name=True):
		if not isfile(path):
			raise FileNotFoundError("No such file '%s'" % path)
		if not access(conf["Paths"]["Files"], W_OK) or not access(conf["Paths"]["Thumbs"], W_OK):
			raise PermissionError("Invalid directories for files and thumbs")
		mime = mage.from_file(path)
		if orig_name == True:
			orig_name = basename(path)
		with _db_cursor() as cur:
			cur.execute("SELECT * FROM tfm_add_file(%s, %s, %s, %s, %s, %s)", (self.sid, mime, datetime, notes, is_private, orig_name))
			res = cur.fetchone()
			file_id = res["f_id"]
			ext = res["ext"]
			file_path = join(conf["Paths"]["Files"], file_id)
			move(path, file_path)
			thumb_path = previewer.get_jpeg_preview(file_path, height=160, width=160)
			preview_path = previewer.get_jpeg_preview(file_path, height=1080, width=1920)
			chmod(file_path, 0o664)
			chmod(thumb_path, 0o664)
			chmod(preview_path, 0o664)
			return file_id, ext

	def add_tag(self, name, notes=None, color=None, category_id=None, is_private=None):
		if color is not None:
			color = color.replace('#', '')
		if not category_id:
			category_id = None
		with _db_cursor() as cur:
			cur.execute("SELECT tfm_add_tag(%s, %s, %s, %s, %s, %s) AS id", (self.sid, name, notes, color, category_id, is_private))
			return cur.fetchone()["id"]

	def add_category(self, name, notes=None, color=None, is_private=None):
		if color is not None:
			color = color.replace('#', '')
		with _db_cursor() as cur:
			cur.execute("SELECT tfm_add_category(%s, %s, %s, %s, %s) AS id", (self.sid, name, notes, color, is_private))
			return cur.fetchone()["id"]

	def add_pool(self, name, notes=None, parent_id=None, is_private=None):
		with _db_cursor() as cur:
			cur.execute("SELECT tfm_add_pool(%s, %s, %s, %s, %s) AS id", (self.sid, name, notes, parent_id, is_private))
			return cur.fetchone()["id"]

	def add_autotag(self, child_id, parent_id, is_active=None, apply_to_existing=None):
		with _db_cursor() as cur:
			cur.execute("SELECT tfm_add_autotag(%s, %s, %s, %s, %s) AS added", (self.sid, child_id, parent_id, is_active, apply_to_existing))
			return cur.fetchone()["added"]

	def add_file_to_tag(self, file_id, tag_id):
		with _db_cursor() as cur:
			cur.execute("SELECT tfm_add_file_to_tag(%s, %s, %s) AS id", (self.sid, file_id, tag_id))
			return list(map(lambda t: t["id"], cur.fetchall()))

	def add_file_to_pool(self, file_id, pool_id):
		with _db_cursor() as cur:
			cur.execute("SELECT tfm_add_file_to_pool(%s, %s, %s) AS added", (self.sid, file_id, pool_id))
			return cur.fetchone()["added"]

	def edit_file(self, file_id, mime=None, datetime=None, notes=None, is_private=None):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_edit_file(%s, %s, %s, %s, %s, %s)", (self.sid, file_id, mime, datetime, notes, is_private))

	def edit_tag(self, tag_id, name=None, notes=None, color=None, category_id=None, is_private=None):
		if color is not None:
			color = color.replace('#', '')
		if not category_id:
			category_id = None
		with _db_cursor() as cur:
			cur.execute("CALL tfm_edit_tag(%s, %s, %s, %s, %s, %s, %s)", (self.sid, tag_id, name, notes, color, category_id, is_private))

	def edit_category(self, category_id, name=None, notes=None, color=None, is_private=None):
		if color is not None:
			color = color.replace('#', '')
		with _db_cursor() as cur:
			cur.execute("CALL tfm_edit_category(%s, %s, %s, %s, %s, %s)", (self.sid, category_id, name, notes, color, is_private))

	def edit_pool(self, pool_id, name=None, notes=None, parent_id=None, is_private=None):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_edit_pool(%s, %s, %s, %s, %s, %s)", (self.sid, pool_id, name, notes, parent_id, is_private))

	def remove_file(self, file_id):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_remove_file(%s, %s)", (self.sid, file_id))
			if system("rm %s/%s*" % (conf["Paths"]["Files"], file_id)):
				raise RuntimeError("Failed to remove file '%s'" % file_id)

	def remove_tag(self, tag_id):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_remove_tag(%s, %s)", (self.sid, tag_id))

	def remove_category(self, category_id):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_remove_category(%s, %s)", (self.sid, category_id))

	def remove_pool(self, pool_id):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_remove_pool(%s, %s)", (self.sid, pool_id))

	def remove_autotag(self, child_id, parent_id):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_remove_autotag(%s, %s, %s)", (self.sid, child_id, parent_id))

	def remove_file_to_tag(self, file_id, tag_id):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_remove_file_to_tag(%s, %s, %s)", (self.sid, file_id, tag_id))

	def remove_file_to_pool(self, file_id, pool_id):
		with _db_cursor() as cur:
			cur.execute("CALL tfm_remove_file_to_pool(%s, %s, %s)", (self.sid, file_id, pool_id))
