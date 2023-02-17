$(window).on("load", function (e) {
	let resp = tdb_query(db_name);
	if (resp == null || !resp.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	$("#stats-sasahyou").append(
		`<tr><td>${new Date(resp.data[0].sasahyou.cts * 1000).toLocaleDateString()} ${new Date(resp.data[0].sasahyou.cts * 1000).toLocaleTimeString()}</td><td>${new Date(resp.data[0].sasahyou.mts * 1000).toLocaleDateString()} ${new Date(resp.data[0].sasahyou.mts * 1000).toLocaleTimeString()}</td><td>${resp.data[0].sasahyou.size}</td><td>${resp.data[0].sasahyou.holes}</td></tr>`
	);
	$("#stats-sappyou").append(
		`<tr><td>${new Date(resp.data[0].sappyou.cts * 1000).toLocaleDateString()} ${new Date(resp.data[0].sappyou.cts * 1000).toLocaleTimeString()}</td><td>${new Date(resp.data[0].sappyou.mts * 1000).toLocaleDateString()} ${new Date(resp.data[0].sappyou.mts * 1000).toLocaleTimeString()}</td><td>${resp.data[0].sappyou.size}</td><td>${resp.data[0].sappyou.holes}</td></tr>`
	);
	$("#stats-shoppyou").append(
		`<tr><td>${new Date(resp.data[0].shoppyou.cts * 1000).toLocaleDateString()} ${new Date(resp.data[0].shoppyou.cts * 1000).toLocaleTimeString()}</td><td>${new Date(resp.data[0].shoppyou.mts * 1000).toLocaleDateString()} ${new Date(resp.data[0].shoppyou.mts * 1000).toLocaleTimeString()}</td><td>${resp.data[0].shoppyou.size}</td><td>${resp.data[0].shoppyou.holes}</td></tr>`
	);
});
