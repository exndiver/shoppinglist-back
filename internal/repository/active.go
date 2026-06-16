package repository

// sqlActive is the soft-delete predicate used as a standalone WHERE clause.
// sqlActiveFrag is the bare fragment appended after an explicit table alias,
// e.g. "g." + sqlActiveFrag → "g.deleted_at IS NULL".
const (
	sqlActive     = "deleted_at IS NULL"
	sqlActiveFrag = "deleted_at IS NULL"
)
