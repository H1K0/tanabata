$(window).on("load", function (e) {
	sappyou_load();
	sappyou.every(tanzaku => {
		$("#content").append(
			`<tr><td>${tanzaku.id}</td><td>${new Date(tanzaku.cts * 1000).toLocaleDateString()} ${new Date(tanzaku.cts * 1000).toLocaleTimeString()}</td><td>${new Date(tanzaku.mts * 1000).toLocaleDateString()} ${new Date(tanzaku.mts * 1000).toLocaleTimeString()}</td><td>${tanzaku.name}</td><td>${tanzaku.desc}</td></tr>`
		);
		return true;
	});
});
