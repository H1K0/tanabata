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
			if (!resp.status) {
				alert(resp.error);
			}
		},
		failure: function (err) {
			$("#loader").css("display", "none");
			alert(err);
		}
	});
});
