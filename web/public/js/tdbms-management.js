db_name = localStorage["db_name"];
if (db_name == null) {
	location.href = "/tdbms/settings";
}

$(window).on("load", function (e) {
	$(".db_name").text(db_name);
});
