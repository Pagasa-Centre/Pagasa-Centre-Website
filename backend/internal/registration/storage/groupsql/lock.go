package groupsql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/registration/domain"
)

func LockGroup(ctx context.Context, tx pgx.Tx, groupID string) (*domain.Group, error) {
	const q = `SELECT ` + GroupSelectCols + `
		  FROM registration_groups
		 WHERE id = $1
		   FOR UPDATE`
	g, err := ScanGroup(tx.QueryRow(ctx, q, groupID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group for update: %w", err)
	}
	return &g, nil
}
