var sasahyou = null, sappyou = null, shoppyou = null;
var current_sasa = null, current_tanzaku = null;
var sasa_modified = false, tanzaku_modified = false;

function sasa_load(id) {
	resp = tdb_query("$TFM", 16, id < 0 ? "" : `${id}`);
	if (resp == null) {
		$(location).attr("href", "/auth");
		throw new Error("Unauthorized");
	}
	if (!resp.status) {
		alert("Something went wrong");
		return;
	}
	if (id < 0) {
		sasahyou = resp.data;
		sasahyou.forEach((sasa) => {
			$(".contents-wrapper").append(`<div class="sasa" id="s${sasa.id}" title="${sasa.path.split('/').slice(-1)}" style="background-image: url(${"/thumbs/" + sasa.path})"><div class="overlay"></div></div>`);
		});
	}
}

function tanzaku_load(id) {
	resp = tdb_query("$TFM", 32, id < 0 ? "" : `${id}`);
	if (resp == null) {
		$(location).attr("href", "/auth");
		throw new Error("Unauthorized");
	}
	if (!resp.status) {
		alert("Something went wrong");
		return;
	}
	if (id < 0) {
		sappyou = resp.data;
		sappyou.forEach((tanzaku) => {
			$("#tanzaku-list").append(`<div class="tanzaku" id="t${tanzaku.id}">${tanzaku.name}</div>`);
		});
	}
}

function kazari_load() {
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

$(window).on("load", function () {
	sasa_load(-1);
	tanzaku_load(-1);
	kazari_load();
});

$(document).keyup(function (e) {
	if (e.key === "Escape") {
		$(".selected").removeClass("selected");
	}
});

$(document).on("click", ".sasa", function (e) {
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

$(document).on("dblclick", ".sasa", function (e) {
	let id = parseInt($(this).attr("id").slice(1));
	sasahyou.every(sasa => {
		if (sasa.id === id) {
			current_sasa = sasa;
			return false;
		}
		return true;
	});
	$(".sasa.selected").removeClass("selected");
	$(".menu-wrapper").css("display", "flex");
	$("#sasa-name").val(decodeURI(current_sasa.path));
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

$(document).on("click", "#btn-close", function (e) {
	e.preventDefault();
	$("#sasa-menu").css("display", "none");
	sappyou.forEach(tanzaku => {
		$(`#t${tanzaku.id}`).removeClass("selected");
	});
});

$(document).on("click", ".tanzaku", function (e) {
	sasa_modified = true;
	if ($(this).hasClass("selected")) {
		$(this).removeClass("selected");
	} else {
		$(this).addClass("selected");
	}
});

$(document).on("input", "#tanzaku-filter", function (e) {
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

$(document).on("click", "#btn-sasa-confirm", function (e) {
	e.preventDefault();
	let resp = tdb_query("$TFM", 24, '' + current_sasa.id);
	if (!resp.status) {
		alert("Something went wrong!");
		return;
	}
	$("#sasa-menu").css("display", "none");
	resp.data.forEach(tanzaku => {
		let current = $(`#t${tanzaku.id}`)
		if (current.hasClass("selected")) {
			current.removeClass("selected");
		} else {
			if (!tdb_query("$TFM", 0b1001, '' + current_sasa.id + ' ' + tanzaku.id).status) {
				console.log("ERROR: failed to remove kazari: " + current_sasa.id + '-' + tanzaku.id);
			}
		}
	});
	$(".tanzaku.selected").each(function (index, element) {
		if (!tdb_query("$TFM", 0b1010, '' + current_sasa.id + ' ' + $(element).attr("id").slice(1))) {
			console.log("ERROR: failed to add kazari: " + current_sasa.id + '-' + tanzaku.id);
		}
	});
	sappyou.forEach(tanzaku => {
		$(`#t${tanzaku.id}`).removeClass("selected");
	});
})
