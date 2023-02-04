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
var current_sasa_index = -1;
var menu_count = 0;

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
	if (menu_count > 1) {
		return;
	}
	menu_count++;
	$("#menu-file-view .selected").removeClass("selected");
	$("#menu-file-view").css("display", "flex");
	$("#preview").attr("src", "/preview/" + current_sasa.path);
	$("#file-name").val(decodeURI(current_sasa.path));
	$("#menu-file-view .list-item").css("display", "");
	$("#btn-full").attr("href", "/files/" + current_sasa.path);
	let resp = tdb_query("$TFM", 24, '' + current_sasa.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(tanzaku => {
		$(`.list-item[tid="${tanzaku.id}"]`).addClass("selected");
	});
	if ($("#file-selection-filter")[0].checked) {
		$("#menu-file-view .list-item:not(.selected)").css("display", "none");
	} else {
		$("#menu-file-view .list-item:not(.selected)").css("display", "block");
	}
}

function menu_view_tag_open() {
	if (menu_count > 1) {
		return;
	}
	menu_count++;
	$(function () {
		$("#menu-tag-view .thumb").Lazy({
			scrollDirection: "vertical",
			effect: "fadeIn",
			visibleOnly: true,
			appendScroll: $("#menu-tag-view .list")[0],
		});
	});
	$("#menu-tag-view .selected").removeClass("selected");
	$("#menu-tag-view").css("display", "flex");
	$("#menu-tag-view .list-item").css("display", "");
	$("#tag-name").val(decodeURI(current_tanzaku.name));
	let resp = tdb_query("$TFM", 40, '' + current_tanzaku.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(sasa => {
		$(`.list-item[sid="${sasa.id}"]`).addClass("selected");
	});
	if ($("#tag-selection-filter")[0].checked) {
		$("#menu-tag-view .list-item:not(.selected)").css("display", "none");
	} else {
		$("#menu-tag-view .list-item:not(.selected)").css("display", "block");
	}
}

function menu_view_file_close() {
	menu_count--;
	$("#menu-file-view").css("display", "none");
	$("#menu-file-view .list-item").removeClass("selected").css("display", "");
	$("#file-name").val("");
	$("#text-filter").val("");
	current_sasa_index = -1;
}

function menu_view_tag_close() {
	menu_count--;
	$("#menu-tag-view").css("display", "none");
	$("#menu-tag-view .list-item").removeClass("selected").css("display", "");
	$("#tag-name").val("");
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

function file_next() {
	if (current_sasa_index === sasahyou.length - 1) {
		menu_view_file_close();
		return;
	}
	current_sasa_index++;
	current_sasa = sasahyou[current_sasa_index];
	menu_count--;
	menu_view_file_open();
}

function file_prev() {
	if (current_sasa_index === 0) {
		menu_view_file_close();
		return;
	}
	current_sasa_index--;
	current_sasa = sasahyou[current_sasa_index];
	menu_count--;
	menu_view_file_open();
}

$(document).keyup(function (e) {
	switch (e.key) {
		case "Esc":
		case "Escape":
			$(".selected").removeClass("selected");
			break;
		case "Left":
		case "ArrowLeft":
			if (current_sasa_index >= 0) {
				file_prev();
			}
			break;
		case "Right":
		case "ArrowRight":
			if (current_sasa_index >= 0) {
				file_next();
			}
			break;
		default:
			return;
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

$(document).on("dblclick", ".sasa", function (e) {
	e.preventDefault();
	let id = parseInt($(this).attr("sid"));
	current_sasa_index = 0;
	sasahyou.every(sasa => {
		if (sasa.id === id) {
			current_sasa = sasa;
			return false;
		}
		current_sasa_index++;
		return true;
	});
	menu_view_file_open();
});

$(document).on("dblclick", ".tanzaku", function (e) {
	e.preventDefault();
	let id = parseInt($(this).attr("tid"));
	sappyou.every(tanzaku => {
		if (tanzaku.id === id) {
			current_tanzaku = tanzaku;
			return false;
		}
		return true;
	});
	menu_view_tag_open();
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

$(document).on("click", "#file-selection-filter", function (e) {
	let notselected = $("#menu-file-view .list-item:not(.selected)");
	if (this.checked) {
		notselected.css("display", "none");
	} else {
		notselected.css("display", "block");
	}
});

$(document).on("click", "#tag-selection-filter", function (e) {
	let notselected = $("#menu-tag-view .list-item:not(.selected)");
	if (this.checked) {
		notselected.css("display", "none");
	} else {
		notselected.css("display", "block");
	}
});

$(document).on("input", "#text-filter", function (e) {
	let filter = $(this).val().toLowerCase();
	let unfiltered;
	if ($("#file-selection-filter")[0].checked) {
		unfiltered = $(".list-item.selected");
	} else {
		unfiltered = $(".list-item");
	}
	if (filter === "") {
		unfiltered.css("display", "");
		return;
	}
	unfiltered.each((index, element) => {
		let current = $(element);
		if (current.text().toLowerCase().includes(filter)) {
			current.css("display", "");
		} else {
			current.css("display", "none");
		}
	});
});

$(document).on("reset", "#menu-file-view form", function (e) {
	e.preventDefault();
	menu_view_file_close();
});

$(document).on("reset", "#menu-tag-view form", function (e) {
	e.preventDefault();
	menu_view_tag_close();
});

$(document).on("reset", "#menu-add form", function (e) {
	e.preventDefault();
	menu_add_close();
});

$(document).on("submit", "#menu-file-view form", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 24, '' + current_sasa.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(tanzaku => {
		let current = $(`.list-item[tid="${tanzaku.id}"]`);
		if (!current.hasClass("selected") &&
			!tdb_query("$TFM", 9, '' + current_sasa.id + ' ' + tanzaku.id).status) {
			console.log("ERROR: failed to remove kazari: " + current_sasa.id + '-' + tanzaku.id);
		}
	});
	$(".list-item.tanzaku.selected").each(function (index, element) {
		let tid = parseInt($(element).attr("tid"));
		if (resp.data.find(t => t.id === tid) != null) {
			return;
		}
		if (!tdb_query("$TFM", 10, '' + current_sasa.id + ' ' + tid)) {
			console.log("ERROR: failed to add kazari: " + current_sasa.id + '-' + tid);
		}
	});
	alert("Saved changes!");
});

$(document).on("submit", "#menu-tag-view form", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 40, '' + current_tanzaku.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(sasa => {
		let current = $(`.list-item[sid="${sasa.id}"]`);
		if (!current.hasClass("selected") &&
			!tdb_query("$TFM", 9, '' + sasa.id + ' ' + current_tanzaku.id).status) {
			console.log("ERROR: failed to remove kazari: " + sasa.id + '-' + current_tanzaku.id);
		}
	});
	$(".list-item.sasa.selected").each(function (index, element) {
		let sid = parseInt($(element).attr("sid"));
		if (resp.data.find(s => s.id === sid) != null) {
			return;
		}
		if (!tdb_query("$TFM", 10, '' + sid + ' ' + current_tanzaku.id)) {
			console.log("ERROR: failed to add kazari: " + sid + '-' + current_tanzaku.id);
		}
	});
	alert("Saved changes!");
});

$(document).on("click", "#btn-remove", function (e) {
	e.preventDefault();
	if (!confirm("This tag will be removed permanently. Are you sure?")) {
		return;
	}
	let resp = tdb_query("$TFM", 33, '' + current_tanzaku.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	menu_add_close();
	location.reload(true);
});

$(document).on("click", "#file-next", function (e) {
	e.preventDefault();
	file_next();
});

$(document).on("click", "#file-prev", function (e) {
	e.preventDefault();
	file_prev();
});
