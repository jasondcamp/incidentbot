package data

import (
	"database/sql"
)

type Incident struct {
	Id		int
	Summary		sql.NullString
	Openedby	sql.NullString
	Commander	sql.NullString
	Manager		sql.NullString
	Severity	sql.NullString
	State		sql.NullString
	Chat_room	sql.NullString
	Created		sql.NullString
	Start		sql.NullString
	End		sql.NullString
}
