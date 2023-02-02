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

function menu_view_file_open() {
	$(".selected").removeClass("selected");
	$(".menu-wrapper").css("display", "flex");
	$("#menu-file-view").css("display", "flex");
	$("#preview").attr("src", "/preview/" + current_sasa.path);
	$("#name").val(decodeURI(current_sasa.path));
	$(".list-item").css("display", "");
	$("#btn-full").attr("href", "/files/" + current_sasa.path);
	let resp = tdb_query("$TFM", 24, '' + current_sasa.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(tanzaku => {
		$(`#t${tanzaku.id}`).addClass("selected");
	});
	if ($("#selection-filter")[0].checked) {
		$(".list-item:not(.selected)").css("display", "none");
	} else {
		$(".list-item:not(.selected)").css("display", "block");
	}
}

function menu_view_tag_open() {
	$(function () {
		$(".thumb").Lazy({
			scrollDirection: "vertical",
			effect: "fadeIn",
			visibleOnly: true,
			appendScroll: $(".list")[0],
		});
	});
	$(".selected").removeClass("selected");
	$(".menu-wrapper").css("display", "flex");
	$("#menu-view").css("display", "flex");
	$("#name").val(decodeURI(current_tanzaku.name));
	let resp = tdb_query("$TFM", 40, '' + current_tanzaku.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(sasa => {
		$(`#s${sasa.id}`).addClass("selected");
	});
	if ($("#selection-filter")[0].checked) {
		$(".list-item:not(.selected)").css("display", "none");
	} else {
		$(".list-item:not(.selected)").css("display", "block");
	}
}

function menu_view_close() {
	$(".menu-wrapper").css("display", "none");
	$("#menu-view").css("display", "none");
	$("#menu-file-view").css("display", "none");
	$(".list-item").removeClass("selected").css("display", "");
	$("#name").val("");
	$(".menu #text-filter").val("");
}

function menu_add_open() {
	$(".menu-wrapper").css("display", "flex");
	$("#menu-add").css("display", "flex");
}

function menu_add_close() {
	$(".menu-wrapper").css("display", "none");
	$("#menu-add").css("display", "none");
	$("#new-name").val("");
	$("#new-description").val("");
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
	menu_add_open();
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
	menu_view_close();
});

$(document).on("click", "#btn-reset", function (e) {
	e.preventDefault();
	menu_add_close();
});
