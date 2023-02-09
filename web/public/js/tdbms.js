function tdb_query(trdb, trc, trb) {
	let output = null;
	$.ajax({
		url: "/TDBMS",
		type: "POST",
		contentType: "application/json",
		data: `{"trdb":${JSON.stringify(trdb)},"trc":${trc},"trb":${JSON.stringify(trb)}}`,
		dataType: "json",
		async: false,
		statusCode: {
			401: function () {
				location.href = "/auth";
				throw new Error("Unauthorized TDBMS request");
			}
		},
		success: function (resp) {
			output = resp;
		},
		failure: function (err) {
			alert(err);
		}
	});
	return output;
}
