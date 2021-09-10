package proxy

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type authSQLite3 struct {
	db *sql.DB
}

// Exists reports whether a user is registered.
func (a authSQLite3) Exists(name string) bool {
	if err := a.init(); err != nil {
		return false
	}
	defer a.close()

	var name2 string
	err := a.db.QueryRow(`SELECT name FROM user WHERE name = ?;`, name).Scan(&name2)
	return err == nil
}

// Passwd returns the SRP salt and verifier of a user or an error.
func (a authSQLite3) Passwd(name string) (salt, verifier []byte, err error) {
	if err = a.init(); err != nil {
		return
	}
	defer a.close()

	a.db.QueryRow(`SELECT salt, verifier FROM user WHERE name = ?;`, name).Scan(&salt, &verifier)
	a.updateTimestamp(name)
	return
}

// SetPasswd creates a password entry if necessary
// and sets the password of a user.
func (a authSQLite3) SetPasswd(name string, salt, verifier []byte) error {
	if err := a.init(); err != nil {
		return err
	}
	defer a.close()

	_, err := a.db.Exec(`REPLACE INTO user (name, salt, verifier) VALUES (?, ?, ?);`, name, salt, verifier)
	if err != nil {
		return err
	}

	a.updateTimestamp(name)
	return nil
}

// Timestamp returns the last time an authentication entry was accessed
// or an error.
func (a authSQLite3) Timestamp(name string) (time.Time, error) {
	if err := a.init(); err != nil {
		return time.Time{}, err
	}
	defer a.close()

	var tstr string
	err := a.db.QueryRow(`SELECT timestamp FROM user WHERE name = ?;`, name).Scan(&tstr)
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse("2006-01-02 15:04:05", tstr)
}

// Import clears the database and and refills it with the passed
// users.
func (a authSQLite3) Import(in []user) {
	if err := a.init(); err != nil {
		return
	}
	defer a.close()

	a.db.Exec(`DELETE FROM user;`)
	for _, u := range in {
		a.SetPasswd(u.name, u.salt, u.verifier)
		a.db.Query(`UPDATE user SET timestamp = ? WHERE name = ?;`, u.timestamp.Format("2006-01-02 15:04:05"), u.name)
	}
}

// Export returns data that can be processed by Import
// or an error.
func (a authSQLite3) Export() ([]user, error) {
	if err := a.init(); err != nil {
		return nil, err
	}
	defer a.close()

	rows, err := a.db.Query(`SELECT * FROM user;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		names = append(names, name)
	}

	var out []user
	for _, name := range names {
		var u user
		u.timestamp, err = a.Timestamp(name)
		if err != nil {
			return nil, err
		}

		u.salt, u.verifier, err = a.Passwd(name)
		if err != nil {
			return nil, err
		}

		out = append(out, u)
	}

	return out, nil
}

func (a authSQLite3) updateTimestamp(name string) {
	a.db.Exec(`UPDATE user SET timestamp = datetime("now") WHERE name = ?;`, name)
}

func (a *authSQLite3) init() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	path := filepath.Dir(executable) + "/auth.sqlite"
	a.db, err = sql.Open("sqlite3", path)
	if err != nil {
		return err
	}

	init := `CREATE TABLE IF NOT EXISTS user (
	name VARCHAR(20) PRIMARY KEY NOT NULL,
	salt BLOB NOT NULL,
	verifier BLOB NOT NULL,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP);`

	if _, err := a.db.Exec(init); err != nil {
		return err
	}

	return nil
}

func (a authSQLite3) close() error {
	return a.db.Close()
}
