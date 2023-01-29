$(window).on("load", function () {
	sasahyou_load();
	sasahyou.forEach((sasa) => {
		$(".contents-wrapper").append(`<div class="item sasa" id="s${sasa.id}" title="${sasa.path.split('/').slice(-1)}" style="background-image: url(${"/thumbs/" + sasa.path})"><div class="overlay"></div></div>`);
	});
	sappyou_load();
	sappyou.forEach((tanzaku) => {
		$(".list").append(`<div class="list-item tanzaku" id="t${tanzaku.id}">${tanzaku.name}</div>`);
	});
});

$(document).on("dblclick", ".item", function (e) {
	let id = parseInt($(this).attr("id").slice(1));
	sasahyou.every(sasa => {
		if (sasa.id === id) {
			current_sasa = sasa;
			return false;
		}
		return true;
	});
	$(".item.selected").removeClass("selected");
	$(".menu-wrapper").css("display", "flex");
	$("#name").val(decodeURI(current_sasa.path));
	$("#btn-full").attr("href", "/files/" + current_sasa.path);
	let resp = tdb_query("$TFM", 24, '' + id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(tanzaku => {
		$(`#t${tanzaku.id}`).addClass("selected");
	});
});

$(document).on("input", "#filter", function (e) {
	let filter = $(this).val().toLowerCase();
	if (filter === "") {
		$(".tanzaku").css("display", "block");
		return;
	}
	sappyou.forEach((tanzaku) => {
		if (tanzaku.name.toLowerCase().includes(filter)) {
			$(`#t${tanzaku.id}`).css("display", "block");
		} else {
			$(`#t${tanzaku.id}`).css("display", "none");
		}
	});
});

$(document).on("click", "#btn-confirm", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 24, '' + current_sasa.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	$(".menu-wrapper").css("display", "none");
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
			console.log("ERROR: failed to add kazari: " + current_sasa.id + '-' + tanzaku.id);
		}
	});
	sappyou.forEach(tanzaku => {
		$(`#t${tanzaku.id}`).removeClass("selected");
	});
})
