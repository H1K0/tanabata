$(window).on("load", function (e) {
	sasahyou_load();
	sasahyou.every(sasa => {
		$("#content").append(
			`<tr><td>${sasa.id}</td><td>${new Date(sasa.cts * 1000).toLocaleDateString()} ${new Date(sasa.cts * 1000).toLocaleTimeString()}</td><td>${sasa.path}</td></tr>`
		);
		return true;
	});
});
