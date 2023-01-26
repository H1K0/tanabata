$(window).on("load", function () {
	let authorized = true;
	if ($.cookie("token") == null) {
		authorized = false;
		$.ajax({
			url: "/token",
			type: "POST",
			contentType: "application/json",
			data: `{"token":"${$.cookie("token")}"}`,
			dataType: "json",
			success: function (resp) {
				if (resp.status) {
					authorized = true;
				}
			},
			failure: function (err) {
				alert(err);
			}
		});
	}
	if (!authorized) {
		$(location).attr("href", "/auth");
	}
});
