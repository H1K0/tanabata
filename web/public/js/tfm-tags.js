$(window).on("load", function () {
	sappyou_load();
	sappyou.forEach((tanzaku) => {
		$(".contents-wrapper").append(`<div class="item tanzaku" tid="${tanzaku.id}">${tanzaku.name}</div>`);
		$("#menu-file-view .list").append(`<div class="list-item tanzaku" tid="${tanzaku.id}">${tanzaku.name}</div>`);
	});
	sasahyou_load();
	sasahyou.forEach((sasa) => {
		$("#menu-tag-view .list").append(`<div class="list-item sasa" sid="${sasa.id}" title="${sasa.path.split('/').slice(-1)}"><img class="thumb" data-src="${"/thumbs/" + sasa.path}"><div class="overlay"></div></div>`);
	});
	lazy_menu = $("#menu-tag-view .thumb").lazy({
		chainable: false,
		scrollDirection: "vertical",
		effect: "fadeIn",
		visibleOnly: true,
		appendScroll: $("#menu-tag-view .list")[0],
	});
});

$(document).on("input", "#text-filter-all", function (e) {
	let filter = $(this).val().toLowerCase();
	let unfiltered = $(".item");
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

$(document).on("submit", "#menu-add form", function (e) {
	e.preventDefault();
	let resp = tdb_query(db_name, 34, $("#new-name").val() + '\n' + $("#new-description").val());
	if (resp == null || !resp.status) {
		alert("Something went wrong!");
		return;
	}
	menu_add_close();
	location.reload(true);
});
