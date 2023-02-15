$(window).on("load", function (e) {
	shoppyou_load();
	shoppyou.every(kazari => {
		$("#content").append(
			`<tr><td>${new Date(kazari.cts * 1000).toLocaleDateString()} ${new Date(kazari.cts * 1000).toLocaleTimeString()}</td><td>${kazari.sasa_id}</td><td>${kazari.tanzaku_id}</td></tr>`
		);
		return true;
	});
});
