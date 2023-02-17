var db_name = localStorage["tfm_db_name"];
if (db_name == null) {
	location.href = "/tfm/settings";
}

$(document).on("click", "#btn-save", function (e) {
	e.preventDefault();
	if (db_name == null) {
		return;
	}
	let resp = tdb_query(db_name, 4);
	if (resp == null || !resp.status) {
		alert("Something went wrong!");
		return;
	}
	alert("Successfully saved!");
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
	if (resp == null || !resp.status) {
		alert("Something went wrong!");
		return;
	}
	localStorage["sasahyou_mts"] = sasahyou_mts = 0;
	localStorage["sappyou_mts"] = sappyou_mts = 0;
	localStorage["shoppyou_mts"] = shoppyou_mts = 0;
	alert("Successfully reloaded database!");
});
