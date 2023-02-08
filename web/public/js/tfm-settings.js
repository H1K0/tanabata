var db_name = localStorage["tfm_db_name"],
	sort_files = localStorage["sort_files"],
	sort_tags = localStorage["sort_tags"];
if (sort_files == null) {
	sort_files = "id";
}
if (sort_tags == null) {
	sort_tags = "id";
}

function settings_load() {
	if (db_name != null) {
		$("#db_name").val(db_name);
	}
	if (sort_files != null) {
		if (sort_files[0] === '-') {
			$("#files-reverse").prop("checked", true);
			sort_files = sort_files.slice(1);
		}
		$(`#files-by-${sort_files}`).prop("checked", true);
	}
	if (sort_tags != null) {
		if (sort_tags[0] === '-') {
			$("#tags-reverse").prop("checked", true);
			sort_tags = sort_tags.slice(1);
		}
		$(`#tags-by-${sort_tags}`).prop("checked", true);
	}
}

$(window).on("load", function () {
	settings_load();
});

$(document).on("reset", "#settings", function (e) {
	e.preventDefault();
	settings_load();
});

$(document).on("submit", "#settings", function (e) {
	e.preventDefault();
	let db_name_input = $("#db_name");
	let db_name_val = db_name_input.val();
	if (db_name_val !== db_name) {
		let resp = tdb_query("", 0, "");
		if (!resp.status) {
			alert("Failed to fetch databases");
			return;
		}
		let found = false;
		resp.data.every(db => {
			if (db.name === db_name_val) {
				db_name = db_name_val;
				localStorage["tfm_db_name"] = db_name;
				found = true;
				db_name_input.removeClass("is-invalid");
				return false;
			}
			return true;
		});
		if (!found) {
			db_name_input.addClass("is-invalid");
			return;
		}
	}
	alert("Successfully updated settings!");
});
