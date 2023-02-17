var db_name = localStorage["tfm_db_name"];
sort_sasa = localStorage["sort_files"];
sort_tanzaku = localStorage["sort_tags"];
if (sort_sasa == null) {
	localStorage["sort_files"] = sort_sasa = "id";
}
if (sort_tanzaku == null) {
	localStorage["sort_tags"] = sort_tanzaku = "id";
}

function settings_load() {
	if (db_name != null) {
		$(`#db_name option[value="${db_name}"]`).prop("selected", true);
	} else {
		$("#db_name option[value=\"\"]").prop("selected", true);
	}
	if (sort_sasa != null) {
		let sort_s = sort_sasa;
		if (sort_s[0] === '!') {
			sort_s = sort_s.slice(1);
		}
		if (sort_s[0] === '-') {
			$("#files-reverse").prop("checked", true);
			sort_s = sort_s.slice(1);
		}
		$(`#files-by-${sort_s}`).prop("checked", true);
	}
	if (sort_tanzaku != null) {
		let sort_t = sort_tanzaku;
		if (sort_t[0] === '!') {
			sort_t = sort_t.slice(1);
		}
		if (sort_t[0] === '-') {
			$("#tags-reverse").prop("checked", true);
			sort_t = sort_t.slice(1);
		}
		$(`#tags-by-${sort_t}`).prop("checked", true);
	}
}

$(window).on("load", function () {
	let resp = tdb_query();
	if (resp == null || !resp.status) {
		alert("Failed to fetch databases");
		throw new Error("Failed to fetch databases");
	}
	resp.data.every(tdb => {
		$("#db_name").append($("<option>", {
			value: tdb.name,
			text: tdb.name
		}));
		return true;
	});
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
		let resp = tdb_query();
		if (resp == null || !resp.status) {
			alert("Failed to fetch databases");
			return;
		}
		let found = false;
		resp.data.every(db => {
			if (db.name === db_name_val) {
				localStorage["tfm_db_name"] = db_name = db_name_val;
				found = true;
				db_name_input.removeClass("is-invalid");
				localStorage["sasahyou_mts"] = sasahyou_mts = 0;
				localStorage["sappyou_mts"] = sappyou_mts = 0;
				localStorage["shoppyou_mts"] = shoppyou_mts = 0;
				return false;
			}
			return true;
		});
		if (!found) {
			db_name_input.addClass("is-invalid");
			return;
		}
	}
	let sort_f = ($("#files-reverse")[0].checked ? '-' : '') + $("input[type=radio][name=sort-files]:checked").attr("id").slice(9);
	let sort_t = ($("#tags-reverse")[0].checked ? '-' : '') + $("input[type=radio][name=sort-tags]:checked").attr("id").slice(8);
	if (sort_f !== sort_sasa && '!' + sort_f !== sort_sasa) {
		localStorage["sort_files"] = sort_sasa = '!' + sort_f;
	}
	if (sort_t !== sort_tanzaku && '!' + sort_t !== sort_tanzaku) {
		localStorage["sort_tags"] = sort_tanzaku = '!' + sort_t;
	}
	alert("Successfully updated settings!");
});
