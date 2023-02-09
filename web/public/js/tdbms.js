var sasahyou = null, sappyou = null, shoppyou = null;
var sort_files = localStorage["sort_files"],
	sort_tags = localStorage["sort_tags"];
if (localStorage["sasahyou"] != null) {
	sasahyou = JSON.parse(localStorage["sasahyou"]);
}
if (localStorage["sappyou"] != null) {
	sappyou = JSON.parse(localStorage["sappyou"]);
}
if (localStorage["shoppyou"] != null) {
	shoppyou = JSON.parse(localStorage["shoppyou"]);
}
var sasahyou_mts = 0, sappyou_mts = 0, shoppyou_mts = 0;
if (localStorage["sasahyou_mts"] != null) {
	sasahyou_mts = parseInt(localStorage["sasahyou_mts"]);
}
if (localStorage["sappyou_mts"] != null) {
	sappyou_mts = parseInt(localStorage["sappyou_mts"]);
}
if (localStorage["shoppyou_mts"] != null) {
	shoppyou_mts = parseInt(localStorage["shoppyou_mts"]);
}
if (sort_files == null) {
	sort_files = "id";
}
if (sort_tags == null) {
	sort_tags = "id";
}

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

function sasahyou_load(tdb) {
	let db_info = tdb_query(tdb, 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (sasahyou == null || sasahyou_mts !== db_info.data[0].sasahyou.mts) {
		let resp = tdb_query(tdb, 16, "");
		if (resp == null || !resp.status) {
			alert("Failed to get sasahyou");
			throw new Error("Failed to get sasahyou");
		}
		sasahyou = resp.data;
		sasahyou_mts = db_info.data[0].sasahyou.mts;
		localStorage["sasahyou"] = JSON.stringify(sasahyou);
		localStorage["sasahyou_mts"] = sasahyou_mts;
	}
}

function sappyou_load(tdb) {
	let db_info = tdb_query(tdb, 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (sappyou == null || sappyou_mts !== db_info.data[0].sappyou.mts) {
		let resp = tdb_query(tdb, 32, "");
		if (resp == null || !resp.status) {
			alert("Failed to get sappyou");
			throw new Error("Failed to get sappyou");
		}
		sappyou = resp.data;
		sappyou_mts = db_info.data[0].sappyou.mts;
		localStorage["sappyou"] = JSON.stringify(sappyou);
		localStorage["sappyou_mts"] = sappyou_mts;
	}
}

function shoppyou_load(tdb) {
	let db_info = tdb_query(tdb, 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (shoppyou == null || shoppyou_mts !== db_info.data[0].shoppyou.mts) {
		let resp = tdb_query(tdb, 8, "");
		if (resp == null || !resp.status) {
			alert("Failed to get shoppyou");
			throw new Error("Failed to get shoppyou");
		}
		shoppyou = resp.data;
		shoppyou_mts = db_info.data[0].shoppyou.mts;
		localStorage["shoppyou"] = JSON.stringify(shoppyou);
		localStorage["shoppyou_mts"] = shoppyou_mts;
	}
}
