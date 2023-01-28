function tdb_query(trdb, trc, trb) {
	output = null;
	$.ajax({
		url: "/TDBMS",
		type: "POST",
		contentType: "application/json",
		data: `{"trdb":"${trdb}","trc":${trc},"trb":"${trb}"}`,
		dataType: "json",
		async: false,
		success: function (resp) {
			output = resp;
		},
		failure: function (err) {
			alert(err);
		}
	});
	return output;
}
