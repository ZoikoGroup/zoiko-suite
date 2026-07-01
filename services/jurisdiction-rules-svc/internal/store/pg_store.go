// Package store provides the PostgreSQL implementation of the jurisdiction
// rules read model.
//
// This package is the ONLY layer that touches the database directly.
// No SQL appears in handlers, service, or domain packages.
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"zoiko.io/jurisdiction-rules-svc/internal/domain"
)

// ListParams controls pagination and filtering for Store.List.
// All filter fields are optional — zero value = no filter applied.
type ListParams struct {
	// JurisdictionType filters by type e.g. "COUNTRY", "STATE_PROVINCE".
	// Empty = return all types.
	JurisdictionType string

	// ActiveOnly = true limits results to active_flag=true and non-expired rows.
	ActiveOnly bool

	// Limit is the page size. 0 defaults to 50; max enforced at 200.
	Limit int

	// Offset is the zero-based page offset.
	Offset int
}

// Store is the interface consumed by the handler.
type Store interface {
	// FindByID returns the Jurisdiction with the given jurisdiction_id.
	// Returns domain.ErrJurisdictionNotFound if no active record exists.
	// Returns domain.ErrStoreUnavailable on any DB error.
	FindByID(ctx context.Context, jurisdictionID string) (*domain.Jurisdiction, error)

	// List returns a paginated slice of jurisdictions matching params.
	// Returns domain.ErrStoreUnavailable on any DB error.
	List(ctx context.Context, params ListParams) ([]*domain.Jurisdiction, error)

	// FindAncestors walks the parent chain starting from jurisdictionID and
	// returns the ordered slice from immediate parent up to the root.
	// The input jurisdiction itself is NOT included in the result.
	// Returns an empty slice (not an error) if jurisdictionID has no parent.
	// Returns domain.ErrJurisdictionNotFound if jurisdictionID itself does not exist.
	// Returns domain.ErrStoreUnavailable on any DB error.
	FindAncestors(ctx context.Context, jurisdictionID string) ([]*domain.Jurisdiction, error)
}

// PgStore implements Store against a PostgreSQL cluster via pgxpool.
type PgStore struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

// New returns an open PgStore. Caller must call pool.Close() when done.
func New(pool *pgxpool.Pool, log *zap.Logger) *PgStore {
	return &PgStore{pool: pool, log: log}
}

// FindByID looks up a jurisdiction by its UUID primary key.
//
// Contract (matching HTTPJurisdictionValidator in tenant-entity-registry-svc):
//   - Returns *Jurisdiction if jurisdiction_id exists AND active_flag = true
//     AND (effective_to IS NULL OR effective_to > NOW()).
//   - Returns domain.ErrJurisdictionNotFound if not found or inactive.
//   - Returns domain.ErrStoreUnavailable on any database error.
func (s *PgStore) FindByID(ctx context.Context, jurisdictionID string) (*domain.Jurisdiction, error) {
	const query = `
		SELECT
			jurisdiction_id,
			jurisdiction_code,
			jurisdiction_name,
			jurisdiction_type,
			parent_jurisdiction_id,
			authority_type,
			effective_from,
			effective_to,
			active_flag,
			created_at,
			created_by_principal_id,
			schema_version
		FROM jurisdictions
		WHERE jurisdiction_id    = $1
		  AND active_flag        = TRUE
		  AND (effective_to IS NULL OR effective_to > NOW())`

	row := s.pool.QueryRow(ctx, query, jurisdictionID)

	j := &domain.Jurisdiction{}
	err := row.Scan(
		&j.JurisdictionID,
		&j.JurisdictionCode,
		&j.JurisdictionName,
		&j.JurisdictionType,
		&j.ParentJurisdictionID,
		&j.AuthorityType,
		&j.EffectiveFrom,
		&j.EffectiveTo,
		&j.ActiveFlag,
		&j.CreatedAt,
		&j.CreatedByPrincipalID,
		&j.SchemaVersion,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrJurisdictionNotFound
		}
		s.log.Error("pg FindByID failed",
			zap.String("jurisdiction_id", jurisdictionID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("%w: %v", domain.ErrStoreUnavailable, err)
	}
	return j, nil
}

// jurisdictionColumns is the standard SELECT column list shared by all queries.
// Order must match scanJurisdiction exactly.
const jurisdictionColumns = `
	jurisdiction_id,
	jurisdiction_code,
	jurisdiction_name,
	jurisdiction_type,
	parent_jurisdiction_id,
	authority_type,
	effective_from,
	effective_to,
	active_flag,
	created_at,
	created_by_principal_id,
	schema_version`

// scanJurisdiction scans one row produced by a jurisdictionColumns SELECT.
func scanJurisdiction(row pgx.Row) (*domain.Jurisdiction, error) {
	j := &domain.Jurisdiction{}
	err := row.Scan(
		&j.JurisdictionID,
		&j.JurisdictionCode,
		&j.JurisdictionName,
		&j.JurisdictionType,
		&j.ParentJurisdictionID,
		&j.AuthorityType,
		&j.EffectiveFrom,
		&j.EffectiveTo,
		&j.ActiveFlag,
		&j.CreatedAt,
		&j.CreatedByPrincipalID,
		&j.SchemaVersion,
	)
	return j, err
}

// List returns a paginated, optionally-filtered slice of jurisdictions.
// Filters are applied via safe positional parameters — no string interpolation
// of user-supplied values.
func (s *PgStore) List(ctx context.Context, params ListParams) ([]*domain.Jurisdiction, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	args := []any{}
	conditions := []string{}
	argIdx := 1

	if params.JurisdictionType != "" {
		conditions = append(conditions, fmt.Sprintf("jurisdiction_type = $%d", argIdx))
		args = append(args, params.JurisdictionType)
		argIdx++
	}
	if params.ActiveOnly {
		conditions = append(conditions,
			"active_flag = TRUE",
			"(effective_to IS NULL OR effective_to > NOW())",
		)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM   jurisdictions
		%s
		ORDER BY jurisdiction_code ASC
		LIMIT  $%d OFFSET $%d`,
		jurisdictionColumns, where, argIdx, argIdx+1,
	)
	args = append(args, limit, params.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.log.Error("pg List failed", zap.Error(err))
		return nil, fmt.Errorf("%w: %v", domain.ErrStoreUnavailable, err)
	}
	defer rows.Close()

	var results []*domain.Jurisdiction
	for rows.Next() {
		j, scanErr := scanJurisdiction(rows)
		if scanErr != nil {
			s.log.Error("pg List scan failed", zap.Error(scanErr))
			return nil, fmt.Errorf("%w: %v", domain.ErrStoreUnavailable, scanErr)
		}
		results = append(results, j)
	}
	if err := rows.Err(); err != nil {
		s.log.Error("pg List rows error", zap.Error(err))
		return nil, fmt.Errorf("%w: %v", domain.ErrStoreUnavailable, err)
	}
	return results, nil
}

// maxAncestorDepth caps the parent-chain walk to prevent runaway on malformed
// data. No real jurisdiction hierarchy exceeds ~5 levels.
const maxAncestorDepth = 20

// FindAncestors walks the parent chain of jurisdictionID iteratively.
// Returns ancestors ordered from immediate parent to root.
// The start jurisdiction itself is NOT included.
// Returns empty slice (not error) when jurisdictionID has no parent.
func (s *PgStore) FindAncestors(ctx context.Context, jurisdictionID string) ([]*domain.Jurisdiction, error) {
	const query = `SELECT` + jurisdictionColumns + `FROM jurisdictions WHERE jurisdiction_id = $1`

	// Confirm starting jurisdiction exists.
	start, err := scanJurisdiction(s.pool.QueryRow(ctx, query, jurisdictionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrJurisdictionNotFound
		}
		s.log.Error("pg FindAncestors: start lookup failed",
			zap.String("jurisdiction_id", jurisdictionID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("%w: %v", domain.ErrStoreUnavailable, err)
	}

	var ancestors []*domain.Jurisdiction
	currentParentID := start.ParentJurisdictionID

	for depth := 0; depth < maxAncestorDepth && currentParentID != nil; depth++ {
		ancestor, scanErr := scanJurisdiction(s.pool.QueryRow(ctx, query, *currentParentID))
		if scanErr != nil {
			if errors.Is(scanErr, pgx.ErrNoRows) {
				// Dangling FK — stop walk, return what we have.
				s.log.Warn("pg FindAncestors: dangling parent reference",
					zap.String("parent_jurisdiction_id", *currentParentID),
				)
				break
			}
			s.log.Error("pg FindAncestors: ancestor lookup failed",
				zap.String("parent_jurisdiction_id", *currentParentID),
				zap.Error(scanErr),
			)
			return nil, fmt.Errorf("%w: %v", domain.ErrStoreUnavailable, scanErr)
		}
		ancestors = append(ancestors, ancestor)
		currentParentID = ancestor.ParentJurisdictionID
	}

	return ancestors, nil
}
