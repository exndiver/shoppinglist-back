package repository

// SQL fragment: only rows not soft-deleted (deleted_at IS NULL).
const sqlActive = "deleted_at IS NULL"
