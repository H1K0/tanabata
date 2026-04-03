package postgres

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Convert "filter" URL param to SQL "WHERE" condition
func filterToSQL(filter string) (sql string, statusCode int, err error) {
	// filterTokens := strings.Split(string(filter), ";")
	sql = "(true)"
	return
}

// Convert "sort" URL param to SQL "ORDER BY"
func sortToSQL(sort string) (sql string, statusCode int, err error) {
	if sort == "" {
		return
	}
	sortOptions := strings.Split(sort, ",")
	sql = " ORDER BY "
	for i, sortOption := range sortOptions {
		sortOrder := sortOption[:1]
		sortColumn := sortOption[1:]
		// parse sorting order marker
		switch sortOrder {
		case "+":
			sortOrder = "ASC"
		case "-":
			sortOrder = "DESC"
		default:
			err = fmt.Errorf("invalid sorting order mark: %q", sortOrder)
			statusCode = http.StatusBadRequest
			return
		}
		// validate sorting column
		var n int
		n, err = strconv.Atoi(sortColumn)
		if err != nil || n < 0 {
			err = fmt.Errorf("invalid sorting column: %q", sortColumn)
			statusCode = http.StatusBadRequest
			return
		}
		// add sorting option to query
		if i > 0 {
			sql += ","
		}
		sql += fmt.Sprintf("%s %s NULLS LAST", sortColumn, sortOrder)
	}
	return
}
