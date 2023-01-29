$(window).on("load", function () {
	sappyou_load();
	sappyou.forEach((tanzaku) => {
		$(".contents-wrapper").append(`<div class="item tanzaku" id="t${tanzaku.id}">${tanzaku.name}</div>`);
	});
	sasahyou_load();
	sasahyou.forEach((sasa) => {
		$(".list").append(`<div class="list-item sasa" id="s${sasa.id}" title="${sasa.path.split('/').slice(-1)}" style="background-image: url(${"/thumbs/" + sasa.path})"><div class="overlay"></div></div>`);
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
	$(".item.selected").removeClass("selected");
	$(".menu-wrapper").css("display", "flex");
	$("#name").val(decodeURI(current_tanzaku.name));
	let resp = tdb_query("$TFM", 40, '' + id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	resp.data.forEach(sasa => {
		$(`#s${sasa.id}`).addClass("selected");
	});
	if ($("#selected")[0].checked) {
		$(".list-item:not(.selected)").css("display", "none");
	} else {
		$(".list-item:not(.selected)").css("display", "block");
	}
});

$(document).on("click", "#btn-confirm", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 40, '' + current_tanzaku.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	$(".menu-wrapper").css("display", "none");
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
	$(".list-item").removeClass("selected").css("display", "block");
});
