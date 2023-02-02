$(window).on("load", function () {
	$(function () {
		$(".thumb").Lazy({
			scrollDirection: "vertical",
			effect: "fadeIn",
			visibleOnly: true,
			appendScroll: $(".contents-wrapper")[0],
		});
	});
	sasahyou_load();
	sasahyou.forEach((sasa) => {
		$(".contents-wrapper").append(`<div class="item sasa" id="s${sasa.id}" title="${sasa.path.split('/').slice(-1)}"><img class="thumb" data-src="${"/thumbs/" + sasa.path}"><div class="overlay"></div></div>`);
	});
	sappyou_load();
	sappyou.forEach((tanzaku) => {
		$(".list").append(`<div class="list-item tanzaku" id="t${tanzaku.id}">${tanzaku.name}</div>`);
	});
});

$(document).on("dblclick", ".item", function (e) {
	e.preventDefault();
	let id = parseInt($(this).attr("id").slice(1));
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

$(document).on("input", "#text-filter", function (e) {
	let filter = $(this).val().toLowerCase();
	let unfiltered;
	if ($("#selection-filter")[0].checked) {
		unfiltered = $(".list-item.selected");
	} else {
		unfiltered = $(".list-item");
	}
	if (filter === "") {
		unfiltered.css("display", "block");
		return;
	}
	unfiltered.each((index, element) => {
		let current = $(element);
		if (current.text().toLowerCase().includes(filter)) {
			current.css("display", "block");
		} else {
			current.css("display", "none");
		}
	});
});

$(document).on("submit", "#menu-file-view form", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 24, '' + current_sasa.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(tanzaku => {
		let current = $(`#t${tanzaku.id}`)
		if (current.hasClass("selected")) {
			current.removeClass("selected");
		} else {
			if (!tdb_query("$TFM", 9, '' + current_sasa.id + ' ' + tanzaku.id).status) {
				console.log("ERROR: failed to remove kazari: " + current_sasa.id + '-' + tanzaku.id);
			}
		}
	});
	$(".tanzaku.selected").each(function (index, element) {
		if (!tdb_query("$TFM", 10, '' + current_sasa.id + ' ' + $(element).attr("id").slice(1))) {
			console.log("ERROR: failed to add kazari: " + current_sasa.id + '-' + $(element).attr("id").slice(1));
		}
	});
	menu_view_close();
});

$(document).on("submit", "#menu-add form", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 18, $("#new-name").val());
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	menu_add_close();
	location.reload(true);
});
