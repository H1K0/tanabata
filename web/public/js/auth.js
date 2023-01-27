$(window).on("load", validate(() => $(".btn-secondary").css("display", "block"), () => {}));

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
				$.cookie("token", resp.token, {expires: 7, path: '/'});
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
