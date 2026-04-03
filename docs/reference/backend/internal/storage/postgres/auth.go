package postgres

import "context"

func UserLogin(ctx context.Context, name, password string) (user_id int, err error) {
	row := connPool.QueryRow(ctx, "SELECT id FROM users WHERE name=$1 AND password=crypt($2, password)", name, password)
	err = row.Scan(&user_id)
	return
}

func UserAuth(ctx context.Context, user_id int) (ok, isAdmin bool) {
	row := connPool.QueryRow(ctx, "SELECT is_admin FROM users WHERE id=$1", user_id)
	err := row.Scan(&isAdmin)
	ok = (err == nil)
	return
}
