var sasahyou = null, sappyou = null, shoppyou = null;
if (localStorage["sasahyou"] != null) {
	sasahyou = JSON.parse(localStorage["sasahyou"]);
}
if (localStorage["sappyou"] != null) {
	sappyou = JSON.parse(localStorage["sappyou"]);
}
if (localStorage["shoppyou"] != null) {
	shoppyou = JSON.parse(localStorage["shoppyou"]);
}
var sasahyou_mts = 0, sappyou_mts = 0, shoppyou_mts = 0;
if (localStorage["sasahyou_mts"] != null) {
	sasahyou_mts = parseInt(localStorage["sasahyou_mts"]);
}
if (localStorage["sappyou_mts"] != null) {
	sappyou_mts = parseInt(localStorage["sappyou_mts"]);
}
if (localStorage["shoppyou_mts"] != null) {
	shoppyou_mts = parseInt(localStorage["shoppyou_mts"]);
}
var current_sasa = null, current_tanzaku = null;

function sasahyou_load() {
	let db_info = tdb_query("$TFM", 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch TFM database");
		throw new Error("Failed to fetch TFM database");
	}
	if (sasahyou == null || sasahyou_mts !== db_info.data[0].sasahyou.mts) {
		let resp = tdb_query("$TFM", 16, "");
		if (resp == null || !resp.status) {
			alert("Failed to get sasahyou");
			throw new Error("Failed to get sasahyou");
		}
		sasahyou = resp.data;
		sasahyou_mts = db_info.data[0].sasahyou.mts;
		localStorage["sasahyou"] = JSON.stringify(sasahyou);
		localStorage["sasahyou_mts"] = sasahyou_mts;
	}
}

function sappyou_load() {
	let db_info = tdb_query("$TFM", 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch TFM database");
		throw new Error("Failed to fetch TFM database");
	}
	if (sappyou == null || sappyou_mts !== db_info.data[0].sappyou.mts) {
		let resp = tdb_query("$TFM", 32, "");
		if (resp == null || !resp.status) {
			alert("Failed to get sappyou");
			throw new Error("Failed to get sappyou");
		}
		sappyou = resp.data;
		sappyou_mts = db_info.data[0].sappyou.mts;
		localStorage["sappyou"] = JSON.stringify(sappyou);
		localStorage["sappyou_mts"] = sappyou_mts;
	}
}

function shoppyou_load() {
	let db_info = tdb_query("$TFM", 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch TFM database");
		throw new Error("Failed to fetch TFM database");
	}
	if (shoppyou == null || shoppyou_mts !== db_info.data[0].shoppyou.mts) {
		let resp = tdb_query("$TFM", 8, "");
		if (resp == null || !resp.status) {
			alert("Failed to get shoppyou");
			throw new Error("Failed to get shoppyou");
		}
		shoppyou = resp.data;
		shoppyou_mts = db_info.data[0].shoppyou.mts;
		localStorage["shoppyou"] = JSON.stringify(shoppyou);
		localStorage["shoppyou_mts"] = shoppyou_mts;
	}
}

$(document).keyup(function (e) {
	if (e.key === "Escape") {
		$(".selected").removeClass("selected");
	}
});

$(document).on("selectstart", ".sasa,.tanzaku", function (e) {
	e.preventDefault();
});

$(document).on("click", ".item", function (e) {
	let wasSelected = $(this).hasClass("selected");
	if (!e.ctrlKey) {
		$(".item.selected").removeClass("selected");
	}
	if (wasSelected) {
		$(this).removeClass("selected");
	} else {
		$(this).addClass("selected");
	}
});

$(document).on("click", "#btn-new", function (e) {
	e.preventDefault();
	$(".menu-wrapper").css("display", "flex");
	$("#menu-add").css("display", "flex");
});

$(document).on("click", ".list-item", function (e) {
	if ($(this).hasClass("selected")) {
		$(this).removeClass("selected");
	} else {
		$(this).addClass("selected");
	}
});

$(document).on("click", "#selection-filter", function (e) {
	if (this.checked) {
		$(".list-item:not(.selected)").css("display", "none");
	} else {
		$(".list-item:not(.selected)").css("display", "block");
	}
});

$(document).on("click", "#btn-close", function (e) {
	e.preventDefault();
	$(".menu-wrapper").css("display", "none");
	$("#menu-view").css("display", "none");
	$(".list-item").removeClass("selected").css("display", "block");
	$("#name").val("");
	$(".menu #text-filter").val("");
});

$(document).on("click", "#btn-reset", function (e) {
	e.preventDefault();
	$(".menu-wrapper").css("display", "none");
	$("#menu-add").css("display", "none");
	$("#new-name").val("");
	$("#new-description").val("");
});
