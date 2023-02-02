$(window).on("load", function () {
	sappyou_load();
	sappyou.forEach((tanzaku) => {
		$(".contents-wrapper").append(`<div class="item tanzaku" id="t${tanzaku.id}">${tanzaku.name}</div>`);
	});
	sasahyou_load();
	sasahyou.forEach((sasa) => {
		$(".list").append(`<div class="list-item sasa" id="s${sasa.id}" title="${sasa.path.split('/').slice(-1)}"><img class="thumb" data-src="${"/thumbs/" + sasa.path}"><div class="overlay"></div></div>`);
	});
});

$(document).on("input", "#text-filter", function (e) {
	let filter = $(this).val().toLowerCase();
	let unfiltered = $(".item");
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

$(document).on("dblclick", ".item", function (e) {
	e.preventDefault();
	let id = parseInt($(this).attr("id").slice(1));
	sappyou.every(tanzaku => {
		if (tanzaku.id === id) {
			current_tanzaku = tanzaku;
			return false;
		}
		return true;
	});
	menu_view_tag_open();
});

$(document).on("submit", "#menu-view form", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 40, '' + current_tanzaku.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(sasa => {
		let current = $(`#s${sasa.id}`)
		if (current.hasClass("selected")) {
			current.removeClass("selected");
		} else {
			if (!tdb_query("$TFM", 9, '' + sasa.id + ' ' + current_tanzaku.id).status) {
				console.log("ERROR: failed to remove kazari: " + sasa.id + '-' + current_tanzaku.id);
			}
		}
	});
	$(".sasa.selected").each(function (index, element) {
		if (!tdb_query("$TFM", 10, '' + $(element).attr("id").slice(1) + ' ' + current_tanzaku.id)) {
			console.log("ERROR: failed to add kazari: " + $(element).attr("id").slice(1) + '-' + current_tanzaku.id);
		}
	});
	menu_add_close();
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

$(document).on("submit", "#menu-add form", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 34, $("#new-name").val() + '\n' + $("#new-description").val());
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	menu_add_close();
	location.reload(true);
});
