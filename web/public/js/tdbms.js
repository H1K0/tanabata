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
	localStorage["sort_files"] = sort_files = "id";
}
if (sort_tags == null) {
	localStorage["sort_tags"] = sort_tags = "id";
}

function tdb_query(trdb, trc, trb) {
	if (trb == null) {
		trb = "";
	}
	if (trc == null) {
		trc = 0;
	}
	if (trdb == null) {
		trdb = "";
	}
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
	let db_info = tdb_query(db_name);
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (sasahyou == null || sasahyou_mts !== db_info.data[0].sasahyou.mts) {
		let resp = tdb_query(db_name, 16);
		if (resp == null || !resp.status) {
			alert("Failed to get sasahyou");
			throw new Error("Failed to get sasahyou");
		}
		sasahyou = resp.data;
		localStorage["sasahyou_mts"] = sasahyou_mts = db_info.data[0].sasahyou.mts;
		localStorage["sasahyou"] = JSON.stringify(sasahyou);
		if (sort_files[0] !== '!') {
			sort_files = '!' + sort_files;
		}
	}
	sasahyou_sort();
}

function sappyou_load() {
	let db_info = tdb_query(db_name);
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (sappyou == null || sappyou_mts !== db_info.data[0].sappyou.mts) {
		let resp = tdb_query(db_name, 32);
		if (resp == null || !resp.status) {
			alert("Failed to get sappyou");
			throw new Error("Failed to get sappyou");
		}
		sappyou = resp.data;
		localStorage["sappyou_mts"] = sappyou_mts = db_info.data[0].sappyou.mts;
		localStorage["sappyou"] = JSON.stringify(sappyou);
		if (sort_tags[0] !== '!') {
			sort_tags = '!' + sort_tags;
		}
	}
	sappyou_sort();
}

function shoppyou_load() {
	let db_info = tdb_query(db_name);
	if (db_info == null || !db_info.status) {
		alert("Failed to fetch database");
		throw new Error("Failed to fetch database");
	}
	if (shoppyou == null || shoppyou_mts !== db_info.data[0].shoppyou.mts) {
		let resp = tdb_query(db_name, 8);
		if (resp == null || !resp.status) {
			alert("Failed to get shoppyou");
			throw new Error("Failed to get shoppyou");
		}
		shoppyou = resp.data;
		localStorage["shoppyou_mts"] = shoppyou_mts = db_info.data[0].shoppyou.mts;
		localStorage["shoppyou"] = JSON.stringify(shoppyou);
	}
}

function sasahyou_sort() {
	if (sort_files[0] !== '!') {
		return;
	}
	let sort = localStorage["sort_files"] = sort_files = sort_files.slice(1);
	let order = 1;
	if (sort[0] === '-') {
		order = -1;
		sort = sort.slice(1);
	}
	sasahyou.sort((lhs, rhs) => {
		let l = lhs[sort], r = rhs[sort];
		if (l > r) {
			return order;
		}
		if (l < r) {
			return -order;
		}
		return 0;
	});
	localStorage["sasahyou"] = JSON.stringify(sasahyou);
}

function sappyou_sort() {
	if (sort_tags[0] !== '!') {
		return;
	}
	let sort = localStorage["sort_tags"] = sort_tags = sort_tags.slice(1);
	let order = 1;
	if (sort[0] === '-') {
		order = -1;
		sort = sort.slice(1);
	}
	if (sort === "nkazari") {
		shoppyou_load();
		shoppyou.every(kazari => {
			sappyou.every((tanzaku, index) => {
				if (tanzaku.id === kazari.tanzaku_id) {
					if (tanzaku.nkazari == null) {
						sappyou[index].nkazari = 1;
					} else {
						sappyou[index].nkazari++;
					}
					return false;
				}
				return true;
			});
			return true;
		});
		sappyou.every((tanzaku, index) => {
			if (tanzaku.nkazari == null) {
				sappyou[index].nkazari = 0;
			}
			return true;
		});
	}
	sappyou.sort((lhs, rhs) => {
		if (lhs.id === 0) {
			return -1;
		}
		if (rhs.id === 0) {
			return 1;
		}
		let l = lhs[sort], r = rhs[sort];
		if (l > r) {
			return order;
		}
		if (l < r) {
			return -order;
		}
		return 0;
	});
	localStorage["sappyou"] = JSON.stringify(sappyou);
}
