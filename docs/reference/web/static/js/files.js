var curr_page = 0;
var load_lock = false;
var init_filter = null;

function files_load() {
	if (load_lock) {
		return;
	}
	load_lock = true;
	let container = $("main");
	$("#loader").css("display", "");
	$.ajax({
		url: "/api/files?limit=50&offset=" + curr_page*50 + (init_filter ? "&filter=" + encodeURIComponent("{" + init_filter + "}") : ""),
		type: "GET",
		contentType: "application/json",
		async: false,
		success: function (resp) {
			$("#loader").css("display", "none");
			resp.forEach((file) => {
				container.append(`<div class="item-preview file-preview" file_id="${file.id}"><img src="/static/thumbs/${file.id}" alt="" class="file-thumb"><div class="overlay"></div></div>`);
			});
			if (resp.length == 50) {
				load_lock = false;
			}
		},
		error: function (xhr, status) {
			$("#loader").css("display", "none");
			alert(xhr.responseText);
			location.href = "/files";
		}
	});
	curr_page++;
}

function tags_load(target) {
	$("#loader").css("display", "");
	$.ajax({
		url: "/api/tags",
		type: "GET",
		contentType: "application/json",
		async: false,
		success: function (resp) {
			$("#loader").css("display", "none");
			resp.forEach((tag) => {
				target.append(`<div class="filtering-token" val="t=${tag.id}" style="background-color: #${tag.category_color}">${escapeHTML(tag.name)}</div>`);
			});
		},
		error: function (xhr, status) {
			$("#loader").css("display", "none");
			alert(status);
		}
	});
}

function filter_load() {
	if (!init_filter) {
		return;
	}
	$("#files-filter").html("");
	let filtering_tokens = init_filter.split(',');
	filtering_tokens.forEach((element) => {
		$(`.filtering-block .filtering-token[val='${element}']`).clone().appendTo("#files-filter");
	});
}

$(window).on("load", function (e) {
	init_filter = /filter=\{([^\}]+)/.exec(decodeURIComponent(location.search));
	init_filter = init_filter ? init_filter[1] : null;
	let container = $("main");
	while (!load_lock && container.scrollTop() + container.innerHeight() >= container[0].scrollHeight) {
		files_load();
	}
	tags_load($("#filtering-tokens-all"));
	filter_load();
});

$("main").scroll(function (e) {
	if ($(this).scrollTop() + $(this).innerHeight() >= $(this)[0].scrollHeight - 100) {
		files_load();
	}
});

$(document).on("click", "#files-filter", function (e) {
	if ($(".filtering-block").is(":hidden")) {
		$(".filtering-block").slideDown("fast");
	}
});

$(document).on("click", "#filtering-apply", function (e) {
	let filtering_tokens = [];
	$("#files-filter > .filtering-token").each((index, element) => {
		filtering_tokens.push($(element).attr("val"));
	});
	location.href = "/files?filter=" + encodeURIComponent("{" + filtering_tokens.join(',') + "}");
});

$(document).on("click", "#filtering-reset", function (e) {
	$(".filtering-block").slideUp("fast");
	filter_load();
	$("#filter-filtering").val("").trigger("input");
});

$(document).on("click", ".filtering-block .filtering-token", function (e) {
	$(this).clone().appendTo("#files-filter");
});

$(document).on("click", "#files-filter > .filtering-token", function (e) {
	$(this).remove();
});

$(document).on("input", "#filter-filtering", function (e) {
	let filter = $(this).val().toLowerCase();
	let unfiltered = $("#filtering-tokens-all > .filtering-token");
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

$(document).on("click", "#filter-filtering", function (e) {
	$(this).val("").trigger("input");
});
