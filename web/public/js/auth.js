$("#auth").on("submit", function submit(e) {
	e.preventDefault();
	var input_password = $("#password");
	let password = input_password.val();
	input_password.val("");
	$.ajax({
		url: "/AUTH",
		type: "POST",
		contentType: "text/plain",
		data: password,
		dataType: "json",
		success: function (resp) {
			if (resp.status) {
				input_password.removeClass("is-invalid");
				input_password.addClass("is-valid");
				$(".btn-secondary").css("display", "block");
			} else {
				input_password.removeClass("is-valid");
				input_password.addClass("is-invalid");
			}
		},
		failure: function (err) {
			alert(err);
		}
	});
});

$(document).keyup(function (e) {
	switch (e.key) {
		case "Esc":
		case "Escape":
			location.href = "/";
			break;
		default:
			return;
	}
});
