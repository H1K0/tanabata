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
		$(".contents-wrapper").append(`<div class="item sasa" sid="${sasa.id}" title="${sasa.path.split('/').slice(-1)}"><img class="thumb" data-src="${"/thumbs/" + sasa.path}"><div class="overlay"></div></div>`);
		$("#menu-tag-view .list").append(`<div class="list-item sasa" sid="${sasa.id}" title="${sasa.path.split('/').slice(-1)}"><img class="thumb" data-src="${"/thumbs/" + sasa.path}"><div class="overlay"></div></div>`);
	});
	sappyou_load();
	sappyou.forEach((tanzaku) => {
		$("#menu-file-view .list").append(`<div class="list-item tanzaku" tid="${tanzaku.id}">${tanzaku.name}</div>`);
	});
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
