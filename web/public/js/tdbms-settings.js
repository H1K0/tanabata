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
			$("#sasa-reverse").prop("checked", true);
			sort_s = sort_s.slice(1);
		}
		$(`#sasa-by-${sort_s}`).prop("checked", true);
	}
	if (sort_tanzaku != null) {
		let sort_t = sort_tanzaku;
		if (sort_t[0] === '!') {
			sort_t = sort_t.slice(1);
		}
		if (sort_t[0] === '-') {
			$("#tanzaku-reverse").prop("checked", true);
			sort_t = sort_t.slice(1);
		}
		$(`#tanzaku-by-${sort_t}`).prop("checked", true);
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
		localStorage["db_name"] = db_name = db_name_val;
		localStorage["sasahyou_mts"] = sasahyou_mts = 0;
		localStorage["sappyou_mts"] = sappyou_mts = 0;
		localStorage["shoppyou_mts"] = shoppyou_mts = 0;
	}
	let sort_s = ($("#sasa-reverse")[0].checked ? '-' : '') + $("input[type=radio][name=sort-sasa]:checked").attr("id").slice(8);
	let sort_t = ($("#tanzaku-reverse")[0].checked ? '-' : '') + $("input[type=radio][name=sort-tanzaku]:checked").attr("id").slice(11);
	if (sort_s !== sort_sasa && '!' + sort_s !== sort_sasa) {
		localStorage["sort_sasa"] = sort_sasa = '!' + sort_s;
	}
	if (sort_t !== sort_tanzaku && '!' + sort_t !== sort_tanzaku) {
		localStorage["sort_tanzaku"] = sort_tanzaku = '!' + sort_t;
	}
	alert("Successfully updated settings!");
});
