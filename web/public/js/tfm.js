var sasahyou, sappyou, shoppyou;

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
		sasahyou = resp.data;
		sasahyou.forEach((sasa) => {
			$(".contents-wrapper").append(`<div class="sasa" id="s${sasa.id}" title="${sasa.path.split('/').slice(-1)}" style="background-image: url(${"/thumbs/" + sasa.path})"><div class="overlay"></div></div>`);
		});
	}
}

function tanzaku_load(id) {
	resp = tdb_query("$TFM", 32, id < 0 ? "" : `${id}`);
	if (resp == null) {
		alert("Unauthorized, go to /auth and authorize");
		return;
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
		alert("Unauthorized, go to /auth and authorize");
		return;
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
	let sasa;
	sasahyou.every(current_sasa => {
		if (current_sasa.id === id) {
			sasa = current_sasa;
			return false;
		}
		return true;
	});
	$(".menu-wrapper").css("display", "flex");
	$("#sasa-name").val(decodeURI(sasa.path));
	$("#btn-full").attr("href", "/files/" + sasa.path);
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
	if ($(this).hasClass("selected")) {
		$(this).removeClass("selected");
	} else {
		$(this).addClass("selected");
	}
});
