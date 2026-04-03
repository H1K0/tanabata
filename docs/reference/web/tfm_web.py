#!../venv/bin/python3

from flask import Flask, render_template, request, session, jsonify, redirect, url_for, send_from_directory, send_file, abort
from flask_cors import CORS
from ua_parser.user_agent_parser import ParseUserAgent
import sys
from os.path import dirname, abspath, join

sys.path.append(dirname(dirname(abspath(__file__))))
import api.tfm_api as tfm_api

tfm_api.Initialize()
app = Flask("TFM")
CORS(app)
app.secret_key = tfm_api.conf["Flask"]["SecretKey"]


@app.route("/api/<func>", methods=["GET"])
def api(func):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		return redirect("/")
	try:
		if func == "files":
			sorting = session.get("sorting")["files"]
			philter = request.args.get("filter")
			offset = request.args.get("offset", type=int, default=0)
			limit = request.args.get("limit", type=int)
			return jsonify(ts.get_files_by_filter(philter, sorting["key"], sorting["asc"], offset, limit))
		if func == "tags":
			sorting = session.get("sorting")["tags"]
			offset = request.args.get("offset", type=int, default=0)
			limit = request.args.get("limit", type=int)
			return jsonify(ts.get_tags(sorting["key"], sorting["asc"], offset, limit))
		if func == "get_my_sessions":
			offset = request.args.get("offset", type=int, default=0)
			limit = request.args.get("limit", type=int)
			return jsonify(ts.get_my_sessions(offset=offset, limit=limit))
		if func == "terminate_session":
			session_id = request.args.get("id");
			if session_id is None:
				session_id = ts.sid
			return jsonify(), 204
		abort(400)
	except Exception as e:
		print(e)
		abort(500, str(e))


@app.route("/favicon.ico")
@app.route("/robots.txt")
@app.route("/tanabata.webmanifest")
@app.route("/browserconfig.xml")
def favicon():
    return send_from_directory(join(app.root_path, "static/service"), request.path[1:])


@app.route("/", methods=["GET"])
def index():
	if session.get("id"):
		return redirect("/files")
	return render_template("auth.html")


@app.route("/auth", methods=["POST"])
def auth():
	try:
		ts = tfm_api.authorize(
			request.form.get("username"),
			request.form.get("password"),
			ParseUserAgent(request.headers.get("User-Agent"))["family"]
		)
	except Exception as e:
		return jsonify({"status": False, "error": str(e).split('\n')[0]})
	else:
		logout()
		session["id"] = ts.sid
		session["sorting"] = tfm_api.DEFAULT_SORTING
		session.permanent = True
		session.modified = True
		return jsonify({"status": True, "is_admin": ts.is_admin})


@app.route("/logout", methods=["GET"])
def logout():
	try:
		ts = tfm_api.TSession(session.get("id"))
		ts.terminate()
	finally:
		session.clear()
		return redirect("/")


@app.route("/files", methods=["GET"])
def files():
	try:
		ts = tfm_api.TSession(session.get("id"))
		sorting = session.get("sorting")["files"]
		sorting_t = session.get("sorting")["tags"]
	except Exception as e:
		logout()
		return redirect("/")
	return render_template("section-files.html",
		files=ts.get_files(sorting["key"], sorting["asc"], limit=100),
		sorting=sorting,
		tags_all=ts.get_tags(sorting_t["key"], sorting_t["asc"])
	)


@app.route("/tags", methods=["GET"])
def tags():
	try:
		ts = tfm_api.TSession(session.get("id"))
		sorting = session.get("sorting")["tags"]
	except Exception as e:
		logout()
		return redirect("/")
	return render_template("section-tags.html",
		tags=ts.get_tags(sorting["key"], sorting["asc"]),
		sorting=sorting
	)


@app.route("/categories", methods=["GET"])
def categories():
	try:
		ts = tfm_api.TSession(session.get("id"))
		sorting = session.get("sorting")["categories"]
	except Exception as e:
		logout()
		return redirect("/")
	return render_template("section-categories.html",
		categories=ts.get_categories(sorting["key"], sorting["asc"]),
		sorting=sorting
	)


@app.route("/settings", methods=["GET"])
def settings():
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		return redirect("/")
	return render_template("section-settings.html")


@app.route("/files/<file_id>", methods=["GET"])
def file(file_id):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		return redirect("/")
	try:
		file = ts.get_file(file_id)
		if not file:
			abort(404, "File does not exist")
		file["datetime"] = file["datetime"].strftime('%Y-%m-%dT%H:%M:%S')
		sorting = session.get("sorting")["tags"]
		ts.view_file(file_id)
		return render_template("view-file.html",
			file=file,
			tags=ts.get_tags_by_file(file_id, sorting["key"], sorting["asc"]),
			tags_all=ts.get_tags(sorting["key"], sorting["asc"])
		)
	except Exception as e:
		abort(400, str(e).split('\n')[0])


@app.route("/tags/<tag_id>", methods=["GET", "POST"])
def tag(tag_id):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		if request.method == "POST":
			abort(401)
		return redirect("/")
	try:
		tag = ts.get_tag(tag_id)
		if not tag:
			raise RuntimeError("Tag does not exist")
		sorting_c = session.get("sorting")["categories"]
		sorting_t = session.get("sorting")["tags"]
		return render_template("view-tag.html",
			tag=tag,
			categories=ts.get_categories(sorting_c["key"], sorting_c["asc"]),
			parent_tags=ts.get_parent_tags(tag_id),
			tags_all=ts.get_tags(sorting_t["key"], sorting_t["asc"])
		)
	except Exception as e:
		abort(400, str(e).split('\n')[0])


@app.route("/categories/<category_id>", methods=["GET", "POST"])
def category(category_id):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		if request.method == "POST":
			abort(401)
		return redirect("/")
	try:
		category = ts.get_category(category_id)
		if not category:
			raise RuntimeError("Category does not exist")
		return render_template("view-category.html",
			category=category
		)
	except Exception as e:
		abort(400, str(e).split('\n')[0])


@app.route("/tags/new", methods=["GET", "POST"])
def new_tag():
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		if request.method == "POST":
			abort(401)
		return redirect("/")
	if request.method == "POST":
		try:
			color = request.form.get("color")
			if color == "#444455":
				color = None
			return jsonify({"status": True, "tag_id": ts.add_tag(request.form.get("name").strip(),
																 request.form.get("notes"),
																 color,
																 request.form.get("category_id"),
																 request.form.get("is_private", False))})
		except Exception as e:
			return jsonify({"status": False, "error": str(e).split('\n')[0]})
	try:
		sorting = session.get("sorting")["categories"]
		return render_template("new-tag.html",
			categories=ts.get_categories(sorting["key"], sorting["asc"])
		)
	except Exception as e:
		abort(400, str(e).split('\n')[0])


@app.route("/categories/new", methods=["GET", "POST"])
def new_category():
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		if request.method == "POST":
			abort(401)
		return redirect("/")
	if request.method == "POST":
		try:
			color = request.form.get("color")
			if color == "#444455":
				color = None
			return jsonify({"status": True, "tag_id": ts.add_category(request.form.get("name").strip(),
																	  request.form.get("notes"),
																	  color,
																	  request.form.get("is_private", False))})
		except Exception as e:
			return jsonify({"status": False, "error": str(e).split('\n')[0]})
	try:
		return render_template("new-category.html")
	except Exception as e:
		abort(400, str(e).split('\n')[0])


@app.route("/files/<file_id>/edit", methods=["POST"])
def edit_file(file_id):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(401)
	try:
		ts.edit_file(file_id, None, request.form.get("datetime"), request.form.get("notes"), request.form.get("is_private", False))
		return jsonify({"status": True})
	except Exception as e:
		return jsonify({"status": False, "error": str(e).split('\n')[0]})


@app.route("/tags/<tag_id>/edit", methods=["POST"])
def edit_tag(tag_id):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(401)
	try:
		color = request.form.get("color")
		if color == "#444455":
			color = ""
		ts.edit_tag(tag_id, request.form.get("name").strip(), request.form.get("notes"), color, request.form.get("category_id"), request.form.get("is_private", False))
		return jsonify({"status": True})
	except Exception as e:
		return jsonify({"status": False, "error": str(e).split('\n')[0]})


@app.route("/categories/<category_id>/edit", methods=["POST"])
def edit_category(category_id):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(401)
	try:
		color = request.form.get("color")
		if color == "#444455":
			color = ""
		ts.edit_category(category_id, request.form.get("name").strip(), request.form.get("notes"), color, request.form.get("is_private", False))
		return jsonify({"status": True})
	except Exception as e:
		return jsonify({"status": False, "error": str(e).split('\n')[0]})


@app.route("/files/<file_id>/tag", methods=["POST"])
def file_tags(file_id):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(401)
	try:
		req = request.get_json()
		if req["add"]:
			return jsonify({"status": True, "tags": ts.add_file_to_tag(file_id, req["tag_id"])})
		else:
			ts.remove_file_to_tag(file_id, req["tag_id"])
			return jsonify({"status": True})
	except Exception as e:
		return jsonify({"status": False, "error": str(e).split('\n')[0]})


@app.route("/tags/<tag_id>/parent", methods=["POST"])
def parent_tags(tag_id):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(401)
	try:
		req = request.get_json()
		if req["add"]:
			ts.add_autotag(tag_id, req["tag_id"])
			return jsonify({"status": True})
		else:
			ts.remove_autotag(tag_id, req["tag_id"])
			return jsonify({"status": True})
	except Exception as e:
		return jsonify({"status": False, "error": str(e).split('\n')[0]})


@app.route("/files/tags", methods=["POST"])
def files_tags():
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(401)
	try:
		req = request.get_json()
		if req["action"] == "get":
			res = set(map(lambda t: t["id"], ts.get_tags_by_file(req["file_id_list"][0])))
			for file_id in req["file_id_list"][1:]:
				res &= set(map(lambda t: t["id"], ts.get_tags_by_file(file_id)))
			return jsonify({"status": True, "tag_id_list": list(res)})
		elif req["action"] == "add":
			res = set()
			for file_id in req["file_id_list"]:
				res |= set(ts.add_file_to_tag(file_id, req["tag_id"]))
			return jsonify({"status": True, "tag_id_list": list(res)})
		elif req["action"] == "remove":
			for file_id in req["file_id_list"]:
				ts.remove_file_to_tag(file_id, req["tag_id"])
			return jsonify({"status": True})
		else:
			return jsonify({"status": False, "error": "unsupported action"})
	except Exception as e:
		return jsonify({"status": False, "error": str(e).split('\n')[0]})


@app.route("/files/delete", methods=["POST"])
def files_delete():
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(401)
	try:
		req = request.get_json()
		for file_id in req["file_id_list"]:
			ts.remove_file(file_id)
		return jsonify({"status": True})
	except Exception as e:
		return jsonify({"status": False, "error": str(e).split('\n')[0]})


@app.route("/static/files/<file_id>", methods=["GET"])
def file_full(file_id=None):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(404)
	try:
		file = ts.get_file(file_id)
		if not file:
			raise RuntimeError("File does not exist")
		return send_file(
			join(tfm_api.conf["Paths"]["Files"], file_id),
			mimetype=file["mime_name"],
			download_name=(file["orig_name"] if file["orig_name"] else "%s.%s" % (file_id, file["extension"]))
		)
	except:
		abort(404)


@app.route("/static/thumbs/<file_id>", methods=["GET"])
def thumb(file_id=None):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(404)
	try:
		file = ts.get_file(file_id)
		if not file:
			raise RuntimeError("File does not exist")
		return send_file(
			tfm_api.previewer.get_jpeg_preview(join(tfm_api.conf["Paths"]["Files"], file_id), height=160, width=160),
			mimetype="image/jpeg"
		)
	except:
		abort(404)


@app.route("/static/previews/<file_id>", methods=["GET"])
def preview(file_id=None):
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		abort(404)
	try:
		file = ts.get_file(file_id)
		if not file:
			raise RuntimeError("File does not exist")
		return send_file(
			tfm_api.previewer.get_jpeg_preview(join(tfm_api.conf["Paths"]["Files"], file_id), height=1080, width=1920),
			mimetype="image/jpeg"
		)
	except:
		abort(404)


@app.route("/settings/sorting", methods=["POST"])
def sorting():
	try:
		ts = tfm_api.TSession(session.get("id"))
	except Exception as e:
		logout()
		return redirect("/")
	req = request.get_json()
	session["sorting"].update(req)
	session.modified = True
	return jsonify({"status": True})


if __name__ == "__main__":
	app.run(host=tfm_api.conf["Flask"]["Host"], port=tfm_api.conf["Flask"]["Port"], debug=True)
