$(document).on("submit", "#newdb", function (e) {
	e.preventDefault();
	let newdb_name = $("#newdb-name").val(), newdb_path = $("#newdb-path").val();
	let resp = tdb_query(newdb_name, 3);
	if (resp == null || !resp.status) {
		alert("Failed to initialize database!");
		return;
	}
	resp = tdb_query(newdb_name, 4, newdb_path);
	if (resp == null || !resp.status) {
		alert("Failed to save database!");
		return;
	}
	resp = tdb_query(newdb_name, 6, "path=" + newdb_path);
	if (resp == null || !resp.status) {
		alert("Failed to finalize database!");
		return;
	}
	alert("Successfully added database!");
});
