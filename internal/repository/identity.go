package repository

import (
	"context"

	"github.com/google/uuid"
)

func UpsertProductIdentity(ctx context.Context, db DBTX, ownerID, productID uuid.UUID, externalID, source string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO product_identities (owner_id, product_id, external_id, source)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (owner_id, external_id, source) DO UPDATE SET
		  product_id = EXCLUDED.product_id
	`, ownerID, productID, externalID, source)
	return err
}

func DeleteIdentityConflictsForMerge(ctx context.Context, db DBTX, ownerID, fromProductID, toProductID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		DELETE FROM product_identities pi
		USING product_identities pi2
		WHERE pi.owner_id = $1
		  AND pi.product_id = $2
		  AND pi2.owner_id = pi.owner_id
		  AND pi2.external_id = pi.external_id
		  AND pi2.source = pi.source
		  AND pi2.product_id = $3
	`, ownerID, fromProductID, toProductID)
	return err
}

func RepointProductIdentities(ctx context.Context, db DBTX, ownerID, fromProductID, toProductID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE product_identities SET product_id = $3 WHERE owner_id = $1 AND product_id = $2
	`, ownerID, fromProductID, toProductID)
	return err
}
