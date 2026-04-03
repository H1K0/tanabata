$(document).on("input", "#parent-tags-filter", function (e) {
	let filter = $(this).val().toLowerCase();
	let unfiltered = $("#parent-tags-other > .tag-preview");
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

$(document).on("click", "#parent-tags-other > .tag-preview", function (e) {
	$("#loader").css("display", "");
	let tag_id = $(this).attr("tag_id");
	$.ajax({
		url: location.pathname + "/parent",
		type: "POST",
		contentType: "application/json",
		data: JSON.stringify({add: true, tag_id: $(this).attr("tag_id")}),
		dataType: "json",
		success: function (resp) {
			$("#loader").css("display", "none");
			if (resp.status) {
				$(`#parent-tags-other > .tag-preview[tag_id='${tag_id}']`).css("display", "none");
				$(`#parent-tags-selected > .tag-preview[tag_id='${tag_id}']`).css("display", "");
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

$(document).on("click", "#parent-tags-selected > .tag-preview", function (e) {
	$("#loader").css("display", "");
	let tag_id = $(this).attr("tag_id");
	$.ajax({
		url: location.pathname + "/parent",
		type: "POST",
		contentType: "application/json",
		data: JSON.stringify({add: false, tag_id: $(this).attr("tag_id")}),
		dataType: "json",
		success: function (resp) {
			$("#loader").css("display", "none");
			if (resp.status) {
				$(`#parent-tags-selected > .tag-preview[tag_id='${tag_id}']`).css("display", "none");
				$(`#parent-tags-other > .tag-preview[tag_id='${tag_id}']`).css("display", "");
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

$(document).on("submit", "#object-edit", function (e) {
	e.preventDefault();
	$("#loader").css("display", "");
	$.ajax({
		url: location.pathname + "/edit",
		type: "POST",
		data: $(this).serialize(),
		dataType: "json",
		success: function (resp) {
			$("#loader").css("display", "none");
			if (resp.status) {
				location.href = location.pathname.substring(0, location.pathname.lastIndexOf("/"));
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
