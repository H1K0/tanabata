var lazy_loader;

function escapeHTML(unsafe) {
	return unsafe
				.replace(/&/g, "&amp;")
				.replace(/</g, "&lt;")
				.replace(/>/g, "&gt;")
				.replace(/"/g, "&quot;")
				.replace(/'/g, "&#039;");
}

function beautify_date(date_string) {
	if (date_string == null) {
		return null;
	}
	let ts = new Date(date_string).getTime();
	let tz = new Date().getTimezoneOffset();
	return new Date(ts-tz*60000).toISOString().slice(0, 19).replace("T", " ");
}

function close_select_manager() {
	$(".item-selected").removeClass("item-selected");
	$(".selection-manager").css("display", "none");
	$("#selection-count").text(0);
	$("main").css("padding-bottom", "");
	$("#selection-tags-other > .tag-preview").css("display", "");
	$("#selection-tags-selected > .tag-preview").css("display", "none");
	$("#selection-tags-filter").val("").trigger("input");
	$(".selection-tags").css("display", "none");
}

function refresh_selection_tags() {
	$("#loader").css("display", "");
	let file_id_list = [];
	$("main > .file-preview.item-selected").each((index, element) => {
		file_id_list.push($(element).attr("file_id"));
	});
	$("#selection-tags-other > .tag-preview").css("display", "");
	$("#selection-tags-selected > .tag-preview").css("display", "none");
	$.ajax({
		url: location.pathname + "/tags",
		type: "POST",
		contentType: "application/json",
		data: JSON.stringify({action: "get", file_id_list: file_id_list}),
		dataType: "json",
		success: function (resp) {
			$("#loader").css("display", "none");
			if (resp.status) {
				resp.tag_id_list.forEach((tag_id) => {
					$(`#selection-tags-other > .tag-preview[tag_id='${tag_id}']`).css("display", "none");
					$(`#selection-tags-selected > .tag-preview[tag_id='${tag_id}']`).css("display", "");
				});
			} else {
				alert(resp.error);
			}
		},
		failure: function (err) {
			$("#loader").css("display", "none");
			alert(err);
		}
	});
}

function select_handler(curr) {
	let selection_count = +$("#selection-count").text();
	if (curr.hasClass("item-selected")) {
		curr.removeClass("item-selected");
		selection_count--;
		$("#selection-count").text(selection_count);
		if (!selection_count) {
			close_select_manager();
			return;
		}
	} else {
		curr.addClass("item-selected");
		$(".selection-manager").css("display", "");
		$("main").css("padding-bottom", "80px");
		selection_count++;
		$("#selection-count").text(selection_count);
	}
	refresh_selection_tags();
}

// $(window).on("load", function () {
// 	lazy_loader = $(".file-thumb").Lazy({
// 		scrollDirection: "vertical",
// 		effect: "fadeIn",
// 		visibleOnly: true,
// 		appendScroll: $("main")[0],
// 		chainable: false,
// 	});
// });

$(document).keyup(function (e) {
	switch (e.key) {
		case "Esc":
		case "Escape":
			close_select_manager();
			break;
		// case "Left":
		// case "ArrowLeft":
		// 	if (current_sasa_index >= 0) {
		// 		file_prev();
		// 	}
		// 	break;
		// case "Right":
		// case "ArrowRight":
		// 	if (current_sasa_index >= 0) {
		// 		file_next();
		// 	}
		// 	break;
		default:
			return;
	}
});

$(document).on("selectstart", ".item-preview", function (e) {
	e.preventDefault();
});

$(document).on("click", "#select", function (e) {
	if ($(".selection-manager").is(":visible")) {
		close_select_manager();
		return;
	}
	$(".selection-manager").css("display", "");
	$("main").css("padding-bottom", "80px");
	selection_count++;
	$("#selection-count").text(selection_count);
});

$(document).on("click", "main > .file-preview", function (e) {
	e.preventDefault();
	if ($(".selection-manager").is(":visible")) {
		select_handler($(this));
		return;
	}
	let id = $(this).attr("file_id");
	$("#viewer").attr("src", "/files/" + id);
	$("#view-prev").attr("file_id", $(this).prev(":visible").attr("file_id"));
	$("#view-next").attr("file_id", $(this).next(":visible").attr("file_id"));
	$(".viewer-wrapper").css("display", "");
});

$(document).on("click", "main > .tag-preview", function (e) {
	e.preventDefault();
	if ($(".selection-manager").is(":visible")) {
		select_handler($(this));
		return;
	}
	let id = $(this).attr("tag_id");
	location.href = "/tags/" + id;
});

$(document).on("click", "main > .category-preview", function (e) {
	e.preventDefault();
	if ($(".selection-manager").is(":visible")) {
		select_handler($(this));
		return;
	}
	let id = $(this).attr("category_id");
	location.href = "/categories/" + id;
});

$(document).on("click", "#sorting", function (e) {
	$("#sorting-options").slideToggle("fast");
	if ($("#sorting-options").is(":visible")) {
		key_prev = $("input[name='sorting'][prev-checked]").val();
		key_curr = $("input[name='sorting']:checked").val();
		asc_prev = $("input[name='order'][prev-checked]").val();
		asc_curr = $("input[name='order']:checked").val();
		if (key_curr != key_prev || asc_curr != asc_prev) {
			$.ajax({
				url: "/settings/sorting",
				type: "POST",
				contentType: "application/json",
				data: JSON.stringify({[location.pathname.split('/')[1]]: {key: key_curr, asc: (asc_curr == "asc")}}),
				dataType: "json",
				success: function (resp) {
					if (resp.status) {
						location.reload();
					} else {
						alert(resp.error);
					}
				},
				failure: function (err) {
					alert(err);
				}
			});
		}
	}
});

$(document).on("input", "#filter", function (e) {
	let filter = $(this).val().toLowerCase();
	let unfiltered = $("main > .item-preview");
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

$(document).on("click", ".filtering", function (e) {
	$(this).val("").trigger("input");
})

$(document).on("click", "#selection-info", function (e) {
	close_select_manager();
});

$(document).on("click", "#selection-edit-tags", function (e) {
	$(".selection-tags").slideToggle("fast");
});

$(document).on("click", "#selection-delete", function (e) {
	if (!confirm("Delete selected?")) {
		return;
	}
	let file_id_list = [];
	$("main > .file-preview.item-selected").each((index, element) => {
		file_id_list.push($(element).attr("file_id"));
	});
	$.ajax({
		url: location.pathname + "/delete",
		type: "POST",
		contentType: "application/json",
		data: JSON.stringify({file_id_list: file_id_list}),
		dataType: "json",
		success: function (resp) {
			$("#loader").css("display", "none");
			if (resp.status) {
				location.reload();
			} else {
				alert(resp.error);
			}
		},
		failure: function (err) {
			$("#loader").css("display", "none");
			alert(err);
		}
	});
});

$(document).on("click", "#selection-tags-other > .tag-preview", function (e) {
	$("#loader").css("display", "");
	let tag_id = $(this).attr("tag_id");
	let file_id_list = [];
	$("main > .file-preview.item-selected").each((index, element) => {
		file_id_list.push($(element).attr("file_id"));
	});
	$.ajax({
		url: location.pathname + "/tags",
		type: "POST",
		contentType: "application/json",
		data: JSON.stringify({action: "add", file_id_list: file_id_list, tag_id: tag_id}),
		dataType: "json",
		success: function (resp) {
			$("#loader").css("display", "none");
			if (resp.status) {
				resp.tag_id_list.forEach((tag_id) => {
					$(`#selection-tags-other > .tag-preview[tag_id='${tag_id}']`).css("display", "none");
					$(`#selection-tags-selected > .tag-preview[tag_id='${tag_id}']`).css("display", "");
				});
			} else {
				alert(resp.error);
			}
		},
		failure: function (err) {
			$("#loader").css("display", "none");
			alert(err);
		}
	});
});

$(document).on("click", "#selection-tags-selected > .tag-preview", function (e) {
	$("#loader").css("display", "");
	let tag_id = $(this).attr("tag_id");
	let file_id_list = [];
	$("main > .file-preview.item-selected").each((index, element) => {
		file_id_list.push($(element).attr("file_id"));
	});
	$.ajax({
		url: location.pathname + "/tags",
		type: "POST",
		contentType: "application/json",
		data: JSON.stringify({action: "remove", file_id_list: file_id_list, tag_id: tag_id}),
		dataType: "json",
		success: function (resp) {
			$("#loader").css("display", "none");
			if (resp.status) {
				$(`#selection-tags-selected > .tag-preview[tag_id='${tag_id}']`).css("display", "none");
				$(`#selection-tags-other > .tag-preview[tag_id='${tag_id}']`).css("display", "");
			} else {
				alert(resp.error);
			}
		},
		failure: function (err) {
			$("#loader").css("display", "none");
			alert(err);
		}
	});
});

$(document).on("input", "#selection-tags-filter", function (e) {
	let filter = $(this).val().toLowerCase();
	let unfiltered = $("#selection-tags-other > .item-preview");
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

$(document).on("scroll", $("#viewer").contents(), function (e) {
	let pos = $(this).scrollTop();
	$(window.parent.document).find(".viewer-nav").css({
		top: (-pos) + "px",
		bottom: pos + "px"
	});
});

$(document).on("click", "#view-next", function (e) {
	let id = $(this).attr("file_id");
	if (!id) {
		return;
	}
	let curr = $(`.file-preview[file_id='${id}']`)
	$("#viewer").attr("src", "/files/" + id);
	$("#view-prev").attr("file_id", curr.prev(":visible").attr("file_id"));
	$("#view-next").attr("file_id", curr.next(":visible").attr("file_id"));
	$(".viewer-wrapper").css("display", "");
});

$(document).on("click", "#view-prev", function (e) {
	let id = $(this).attr("file_id");
	if (!id) {
		return;
	}
	let curr = $(`.file-preview[file_id='${id}']`)
	$("#viewer").attr("src", "/files/" + id);
	$("#view-prev").attr("file_id", curr.prev(":visible").attr("file_id"));
	$("#view-next").attr("file_id", curr.next(":visible").attr("file_id"));
	$(".viewer-wrapper").css("display", "");
});

$(document).on("click", "#view-close", function (e) {
	$(".viewer-wrapper").css("display", "none");
});
