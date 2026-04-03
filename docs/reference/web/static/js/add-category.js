$(document).on("submit", "#object-add", function (e) {
	e.preventDefault();
	$("#loader").css("display", "");
	$.ajax({
		url: location.pathname,
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
