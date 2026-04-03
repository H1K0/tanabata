$("#auth").on("submit", function submit(e) {
	e.preventDefault();
	$.ajax({
		url: "/auth",
		type: "POST",
		data: $("#auth").serialize(),
		dataType: "json",
		success: function(resp) {
			if (resp.status) {
				location.reload();
			} else {
				alert(resp.error);
			}
		},
		failure: function(err) {
			alert(err);
		}
	});
});
