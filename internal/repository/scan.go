package repository

// RowScanner abstracts pgx.Row and pgx.Rows (after Next).
type RowScanner interface {
	Scan(dest ...any) error
}
