var db_name = localStorage["db_name"];

$(window).on("load", function (e) {
	if (db_name != null) {
		$(".db_name").text(db_name);
	}
});
