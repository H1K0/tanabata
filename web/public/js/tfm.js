$(window).on("load", function () {
	sasa_load(-1);
})

$(document).keyup(function (e) {
	if (e.key === "Escape") {
		$(".selected").removeClass("selected");
	}
});

$(document).on("click", ".item", function (e) {
	let wasSelected = $(this).hasClass("selected");
	if (!e.ctrlKey) {
		$(".selected").removeClass("selected");
		wasSelected = false;
	}
	if (wasSelected) {
		$(this).removeClass("selected");
	} else {
		$(this).addClass("selected");
	}
});

function sasa_load(id) {
	resp = tdb_query("$TFM", 16, id < 0 ? "" : `${id}`);
	if (resp == null) {
		alert("Unauthorized, go to /auth and authorize");
		return;
	}
	if (!resp.status) {
		alert("Something went wrong");
		return;
	}
	if (id < 0) {
		resp.data.forEach((sasa) => {
			$(".contents-wrapper").append(`<div class="item" id="s${sasa.id}" title="${sasa.path.split('/').slice(-1)}" style="background-image: url(${"/thumbs/" + sasa.path})"><div class="overlay"></div></div>`);
		});
	}
}
