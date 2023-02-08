var db_name = localStorage["tfm_db_name"];
if (db_name == null) {
	location.href = "/tfm/settings";
}

$(document).on("click", "#btn-save", function (e) {
	e.preventDefault();
	let resp = tdb_query(db_name, 4, "");
	if (resp.status) {
		alert("Successfully saved!");
	} else {
		alert("Something went wrong!");
	}
});

$(document).on("click", "#btn-discard", function (e) {
	e.preventDefault();
	if (!confirm("All unsaved changes will be lost permanently. Are you sure?")) {
		return;
	}
	let resp = tdb_query(db_name, 2, "");
	if (resp.status) {
		alert("Successfully reloaded database!");
	} else {
		alert("Something went wrong!");
	}
});
