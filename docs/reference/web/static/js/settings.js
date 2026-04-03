$(window).on("load", function () {
	$.ajax({
		url: "/api/get_my_sessions",
		type: "GET",
		contentType: "application/json",
		success: function (resp) {
			let timezone_offset = new Date().getTimezoneOffset();
			resp.forEach((session) => {
				let s_started = beautify_date(session.started);
				let s_expires = beautify_date(session.expires);
				$("#sessions-table").append(`<tr><td>${session.user_agent_name}</td><td>${s_started}</td><td>${s_expires === null ? "-" : session.expires}</td><td align="right"><img src="/static/images/icon-terminate.svg" alt="Terminate" class="btn-terminate" session_id="${session.id}"></td></tr>`);
			});
		},
		failure: function (err) {
			alert(err);
		}
	});
});
