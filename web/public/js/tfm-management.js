db_name = localStorage["tfm_db_name"];
if (db_name == null) {
	location.href = "/tfm/settings";
}
sort_sasa = localStorage["sort_files"];
sort_tanzaku = localStorage["sort_tags"];
if (sort_sasa == null) {
	localStorage["sort_files"] = sort_sasa = "id";
}
if (sort_tanzaku == null) {
	localStorage["sort_tags"] = sort_tanzaku = "id";
}
var current_sasa = null, current_tanzaku = null;
var current_sasa_index = -1;
var menu_count = 0;

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
	let resp = tdb_query(db_name, 24, '' + current_sasa.id);
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
	$("#description").val(current_tanzaku.desc);
	let resp = tdb_query(db_name, 40, '' + current_tanzaku.id);
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
	$("#description").val("");
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
	let resp = tdb_query(db_name, 24, '' + current_sasa.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	let toadd = "", toremove = "";
	resp.data.forEach(tanzaku => {
		let current = $(`.list-item[tid="${tanzaku.id}"]`);
		if (!current.hasClass("selected")) {
			toremove += ' ' + tanzaku.id;
		}
	});
	$(".list-item.tanzaku.selected").each(function (index, element) {
		let tid = parseInt($(element).attr("tid"));
		if (resp.data.find(t => t.id === tid) == null) {
			toadd += ' ' + tid;
		}
	});
	let status = true;
	if (toadd !== "" && !tdb_query(db_name, 26, '' + current_sasa.id + toadd).status) {
		status = false;
	}
	if (toremove !== "" && !tdb_query(db_name, 25, '' + current_sasa.id + toremove).status) {
		status = false;
	}
	if (status) {
		alert("Saved changes!");
	} else {
		alert("Something went wrong!");
	}
});

$(document).on("submit", "#menu-tag-view form", function (e) {
	e.preventDefault();
	let resp;
	let name = $("#tag-name").val(),
		desc = $("#description").val();
	if (name !== current_tanzaku.name || desc !== current_tanzaku.desc) {
		resp = tdb_query(db_name, 36, '' + current_tanzaku.id + ' ' + name + '\n' + desc);
		if (!resp.status) {
			alert("Something went wrong!");
			return;
		}
		current_tanzaku.name = name;
		current_tanzaku.desc = desc;
		$(`.tanzaku[tid=${current_tanzaku.id}]`).text(name);
	}
	resp = tdb_query(db_name, 40, '' + current_tanzaku.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	let toadd = "", toremove = "";
	resp.data.forEach(sasa => {
		let current = $(`.list-item[sid="${sasa.id}"]`);
		if (!current.hasClass("selected")) {
			toremove += ' ' + sasa.id;
		}
	});
	$(".list-item.sasa.selected").each(function (index, element) {
		let sid = parseInt($(element).attr("sid"));
		if (resp.data.find(s => s.id === sid) == null) {
			toadd += ' ' + sid;
		}
	});
	let status = true;
	if (toadd !== "" && !tdb_query(db_name, 42, '' + current_tanzaku.id + toadd).status) {
		status = false;
	}
	if (toremove !== "" && !tdb_query(db_name, 41, '' + current_tanzaku.id + toremove).status) {
		status = false;
	}
	if (status) {
		alert("Saved changes!");
	} else {
		alert("Something went wrong!");
	}
});

$(document).on("click", "#btn-remove", function (e) {
	e.preventDefault();
	if (!confirm("This tag will be removed permanently. Are you sure?")) {
		return;
	}
	let resp = tdb_query(db_name, 33, '' + current_tanzaku.id);
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
