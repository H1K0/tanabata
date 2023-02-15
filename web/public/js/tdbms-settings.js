var db_name = localStorage["db_name"];

function settings_load() {
	if (db_name != null) {
		$("#db_name").val(db_name);
	} else {
		$("#db_name").val("");
	}
	if (sort_sasa != null) {
		if (sort_sasa[0] === '-') {
			$("#sasa-reverse").prop("checked", true);
			sort_sasa = sort_sasa.slice(1);
		}
		$(`#sasa-by-${sort_sasa}`).prop("checked", true);
	}
	if (sort_tanzaku != null) {
		if (sort_tanzaku[0] === '-') {
			$("#tanzaku-reverse").prop("checked", true);
			sort_tanzaku = sort_tanzaku.slice(1);
		}
		$(`#tanzaku-by-${sort_tanzaku}`).prop("checked", true);
	}
}

$(window).on("load", function () {
	settings_load();
	if (db_name != null) {
		$(".db_name").text(db_name);
	}
	let resp = tdb_query();
	if (!resp.status) {
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
