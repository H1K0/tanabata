$(document).on("click", "#btn-save", function (e) {
	e.preventDefault();
	if (db_name == null) {
		return;
	}
	let resp = tdb_query(db_name, 4);
	if (resp.status) {
		alert("Successfully saved!");
	} else {
		alert("Something went wrong!");
	}
});

$(document).on("click", "#btn-reload", function (e) {
	e.preventDefault();
	if (db_name == null) {
		return;
	}
	if (!confirm("All unsaved changes will be lost permanently. Are you sure?")) {
		return;
	}
	let resp = tdb_query(db_name, 2);
	if (resp.status) {
		localStorage["sasahyou_mts"] = sasahyou_mts = 0;
		localStorage["sappyou_mts"] = sappyou_mts = 0;
		localStorage["shoppyou_mts"] = shoppyou_mts = 0;
		alert("Successfully reloaded database!");
	} else {
		alert("Something went wrong!");
	}
});

$(document).on("click", "#btn-remove", function (e) {
	e.preventDefault();
	if (db_name == null) {
		return;
	}
	if (!confirm(`Are you sure want to remove database "${db_name}"?`)) {
		return;
	}
	let resp = tdb_query(db_name, 1);
	if (resp.status) {
		localStorage.removeItem("db_name");
		db_name = null;
		localStorage["sasahyou_mts"] = sasahyou_mts = 0;
		localStorage["sappyou_mts"] = sappyou_mts = 0;
		localStorage["shoppyou_mts"] = shoppyou_mts = 0;
		alert("Successfully removed database!");
	} else {
		alert("Something went wrong!");
	}
});
