package repository

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
)

func UpsertGoodIdentity(ctx context.Context, db DBTX, ownerID, goodID uuid.UUID, externalID, source string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO good_identities (owner_id, good_id, external_id, source)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (owner_id, external_id, source) DO UPDATE SET
		  good_id = EXCLUDED.good_id
	`, ownerID, goodID, externalID, source)
	return err
}

func DeleteIdentityConflictsForMerge(ctx context.Context, db DBTX, ownerID, fromGoodID, toGoodID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		DELETE FROM good_identities pi
		USING good_identities pi2
		WHERE pi.owner_id = $1
		  AND pi.good_id = $2
		  AND pi2.owner_id = $1
		  AND pi2.external_id = pi.external_id
		  AND pi2.source = pi.source
		  AND pi2.good_id = $3
	`, ownerID, fromGoodID, toGoodID)
	return err
}

func RepointGoodIdentities(ctx context.Context, db DBTX, ownerID, fromGoodID, toGoodID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE good_identities SET good_id = $3 WHERE owner_id = $1 AND good_id = $2
	`, ownerID, fromGoodID, toGoodID)
	return err
}

func ListGoodIdentities(ctx context.Context, db DBTX, ownerID, goodID uuid.UUID) ([]models.GoodIdentity, error) {
	rows, err := db.Query(ctx, `
		SELECT source, external_id
		FROM good_identities
		WHERE owner_id = $1 AND good_id = $2
		ORDER BY source ASC, external_id ASC
	`, ownerID, goodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.GoodIdentity
	for rows.Next() {
		var id models.GoodIdentity
		if err := rows.Scan(&id.Source, &id.ExternalID); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
