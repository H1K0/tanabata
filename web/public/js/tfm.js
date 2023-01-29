var sasahyou = null, sappyou = null, shoppyou = null;
var current_sasa = null, current_tanzaku = null;
var sasa_modified = false, tanzaku_modified = false;

function sasahyou_load() {
	resp = tdb_query("$TFM", 16, "");
	if (resp == null) {
		$(location).attr("href", "/auth");
		throw new Error("Unauthorized");
	}
	if (!resp.status) {
		alert("Something went wrong");
		return;
	}
	sasahyou = resp.data;
}

function sappyou_load() {
	resp = tdb_query("$TFM", 32, "");
	if (resp == null) {
		$(location).attr("href", "/auth");
		throw new Error("Unauthorized");
	}
	if (!resp.status) {
		alert("Something went wrong");
		return;
	}
	sappyou = resp.data;
}

function shoppyou_load() {
	resp = tdb_query("$TFM", 8, "");
	if (resp == null) {
		$(location).attr("href", "/auth");
		throw new Error("Unauthorized");
	}
	if (!resp.status) {
		alert("Something went wrong");
		return;
	}
	shoppyou = resp.data;
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

$(document).on("click", ".list-item", function (e) {
	if ($(this).hasClass("selected")) {
		$(this).removeClass("selected");
	} else {
		$(this).addClass("selected");
	}
});

$(document).on("click", "#btn-close", function (e) {
	e.preventDefault();
	$(".menu-wrapper").css("display", "none");
	$(".list-item").removeClass("selected");
});
