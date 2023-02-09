var db_name = null;
var sasahyou = localStorage["sasahyou"],
	sappyou = localStorage["sappyou"],
	shoppyou = localStorage["shoppyou"];
var sort_files = localStorage["sort_files"],
	sort_tags = localStorage["sort_tags"];
if (sasahyou != null) {
	sasahyou = JSON.parse(sasahyou);
}
if (sappyou != null) {
	sappyou = JSON.parse(sappyou);
}
if (shoppyou != null) {
	shoppyou = JSON.parse(shoppyou);
}
var sasahyou_mts = localStorage["sasahyou_mts"],
	sappyou_mts = localStorage["sappyou_mts"],
	shoppyou_mts = localStorage["shoppyou_mts"];
if (sasahyou_mts != null) {
	sasahyou_mts = parseInt(sasahyou_mts);
}
if (sappyou_mts != null) {
	sappyou_mts = parseInt(sappyou_mts);
}
if (shoppyou_mts != null) {
	shoppyou_mts = parseInt(shoppyou_mts);
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

function sasahyou_load() {
	let db_info = tdb_query(db_name, 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (sasahyou == null || sasahyou_mts !== db_info.data[0].sasahyou.mts) {
		let resp = tdb_query(db_name, 16, "");
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

function sappyou_load() {
	let db_info = tdb_query(db_name, 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (sappyou == null || sappyou_mts !== db_info.data[0].sappyou.mts) {
		let resp = tdb_query(db_name, 32, "");
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

function shoppyou_load() {
	let db_info = tdb_query(db_name, 0, "");
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (shoppyou == null || shoppyou_mts !== db_info.data[0].shoppyou.mts) {
		let resp = tdb_query(db_name, 8, "");
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
