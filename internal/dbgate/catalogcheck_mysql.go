// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package dbgate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// mysqlCatalogName is the only shape a name may have to be inlined into the lookup, which every allowlisted name has.
var mysqlCatalogName = regexp.MustCompile(`^[a-z0-9_]+$`)

// mysqlCatalogRoutines finds a stored function sharing a called name. The server's own schemas are left out because MySQL 8 ships sys.format_bytes and sys.ps_thread_id, read-only formatters that would otherwise refuse the performance-view helpers on every server. ROUTINES lists only routines the user holds a privilege on, and calling one needs EXECUTE, so every function the channel could reach is listed.
const mysqlCatalogRoutines = `SELECT LOWER(ROUTINE_NAME) FROM information_schema.ROUTINES WHERE ROUTINE_TYPE = 'FUNCTION' AND ROUTINE_SCHEMA NOT IN ('sys', 'mysql', 'performance_schema', 'information_schema') AND LOWER(ROUTINE_NAME) IN (%s)`

// mysqlCatalogLoadable finds a loadable function sharing a called name. MariaDB has no such table, and a user may lack SELECT on it, so in both cases only the routines half runs.
const mysqlCatalogLoadable = ` UNION ALL SELECT LOWER(UDF_NAME) FROM performance_schema.user_defined_functions WHERE LOWER(UDF_NAME) IN (%s)`

// checkMySQLCatalog refuses a statement when any function it calls by an allowlisted name could resolve to a stored or loadable function instead of the built-in, as happens when the server lacks that built-in. It runs inside tx before the caller's SQL and fails closed: a lookup that errors refuses the statement.
func checkMySQLCatalog(tx *sql.Tx, stmt Statement) error {
	names := stmt.Catalog.Functions
	if len(names) == 0 {
		return nil
	}
	quoted := make([]string, len(names))
	for i, name := range names {
		if !mysqlCatalogName.MatchString(name) {
			return &ErrFunctionNotAllowed{Name: name}
		}
		quoted[i] = "'" + name + "'"
	}
	list := strings.Join(quoted, ", ")
	routines := fmt.Sprintf(mysqlCatalogRoutines, list)

	found, err := mysqlCatalogLookup(tx, routines+fmt.Sprintf(mysqlCatalogLoadable, list))
	if err != nil {
		if !mysqlLoadableDenied(err) {
			mariaDB, verr := mysqlIsMariaDB(tx)
			if verr != nil || !mariaDB {
				return mysqlCatalogFailed(err)
			}
		}
		if found, err = mysqlCatalogLookup(tx, routines); err != nil {
			return mysqlCatalogFailed(err)
		}
	}
	if found != "" {
		return &ErrFunctionNotAllowed{Name: found}
	}
	return nil
}

// mysqlLoadableDenied reports a user without SELECT on performance_schema.user_defined_functions. Falling back to ROUTINES alone then leaves the gap MariaDB already has: installing a loadable function needs an admin to place a plugin file, and MySQL refuses one named like a built-in, so only an allowlisted name the server lacks stays unchecked. information_schema never answers 1142, so the error can only come from the loadable half.
func mysqlLoadableDenied(err error) bool {
	var myErr *mysql.MySQLError
	return errors.As(err, &myErr) && myErr.Number == mysqlTableAccessDenied
}

// mysqlTableAccessDenied is ER_TABLEACCESS_DENIED_ERROR.
const mysqlTableAccessDenied = 1142

// mysqlFunctionCheckNotice tells the human, at connection test, that the connection's user can't read user_defined_functions and names the grant that restores the full check. MariaDB has no such table and answers 1146, so it gets no notice. Any other probe failure gives no notice, since the test itself already passed.
func mysqlFunctionCheckNotice(db *sql.DB) *TestNotice {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	_, err := db.ExecContext(ctx, "SELECT 1 FROM performance_schema.user_defined_functions LIMIT 0")
	if !mysqlLoadableDenied(err) {
		return nil
	}
	grantee := "<user>"
	var current string
	if db.QueryRowContext(ctx, "SELECT CURRENT_USER()").Scan(&current) == nil {
		if user, host, ok := cutLast(current, "@"); ok {
			grantee = "'" + user + "'@'" + host + "'"
		}
	}
	return &TestNotice{
		Title:  "Functions are checked against stored functions only",
		Detail: "This user cannot read performance_schema.user_defined_functions, so loadable functions go unchecked. Run this grant to check them too.",
		Fix:    "GRANT SELECT ON performance_schema.user_defined_functions TO " + grantee + ";",
	}
}

func cutLast(s, sep string) (string, string, bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+len(sep):], true
}

func mysqlCatalogFailed(err error) error {
	return fmt.Errorf("dbgate: function check failed, the statement was not run: %s", describeDBError(err))
}

// mysqlCatalogLookup returns the first name query finds, or "" when it finds none.
func mysqlCatalogLookup(tx *sql.Tx, query string) (string, error) {
	var name string
	err := tx.QueryRow(query + " LIMIT 1").Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// mysqlIsMariaDB is asked only after the full lookup failed, so a MySQL server pays no extra round trip.
func mysqlIsMariaDB(tx *sql.Tx) (bool, error) {
	var version string
	if err := tx.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(version), "mariadb"), nil
}
