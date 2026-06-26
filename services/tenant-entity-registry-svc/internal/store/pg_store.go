// Package store provides the PostgreSQL implementation of registry.Store.
//
// This is a scaffold — every method has the correct signature and a structured
// log call so the build compiles and tests run immediately. Replace the stub
// bodies with real pgx queries as the DB schema is applied.
//
// Schema migrations live in deployments/migrations/ (to be created in a
// follow-up task). All tables follow the keying strategy in data-model §03.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"zoiko.io/tenant-entity-registry-svc/internal/domain"
)

// PgStore implements registry.Store against a PostgreSQL cluster via pgxpool.
type PgStore struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

// New returns an open PgStore. Caller must call Close() when done.
func New(pool *pgxpool.Pool, log *zap.Logger) *PgStore {
	return &PgStore{pool: pool, log: log}
}

// Close releases the connection pool.
func (s *PgStore) Close() {
	s.pool.Close()
}

// ---------------------------------------------------------------------------
// Tenant
// ---------------------------------------------------------------------------

func (s *PgStore) CreateTenant(ctx context.Context, t *domain.Tenant) error {
	s.log.Debug("store.CreateTenant (stub)", zap.String("tenant_id", t.TenantID))
	// TODO: INSERT INTO tenants (...) VALUES (...) ON CONFLICT (tenant_code) DO NOTHING
	return nil
}

func (s *PgStore) GetTenantByID(ctx context.Context, tenantID string) (*domain.Tenant, error) {
	s.log.Debug("store.GetTenantByID (stub)", zap.String("tenant_id", tenantID))
	// TODO: SELECT ... FROM tenants WHERE tenant_id = $1
	return nil, fmt.Errorf("store.GetTenantByID: not implemented")
}

func (s *PgStore) TransitionTenantLifecycle(ctx context.Context, tenantID string, newState domain.TenantLifecycleState, actorID, correlationID string) error {
	s.log.Debug("store.TransitionTenantLifecycle (stub)",
		zap.String("tenant_id", tenantID),
		zap.String("new_state", string(newState)),
	)
	// TODO: UPDATE tenants SET lifecycle_state = $1 WHERE tenant_id = $2
	return nil
}

// ---------------------------------------------------------------------------
// LegalEntity
// ---------------------------------------------------------------------------

func (s *PgStore) CreateEntity(ctx context.Context, e *domain.LegalEntity) error {
	s.log.Debug("store.CreateEntity (stub)", zap.String("legal_entity_id", e.LegalEntityID))
	// TODO: INSERT INTO legal_entities (...) VALUES (...)
	return nil
}

func (s *PgStore) GetEntityByID(ctx context.Context, legalEntityID string) (*domain.LegalEntity, error) {
	s.log.Debug("store.GetEntityByID (stub)", zap.String("legal_entity_id", legalEntityID))
	// TODO: SELECT ... FROM legal_entities WHERE legal_entity_id = $1
	return nil, fmt.Errorf("store.GetEntityByID: not implemented")
}

func (s *PgStore) ListEntitiesByTenant(ctx context.Context, tenantID string) ([]*domain.LegalEntity, error) {
	s.log.Debug("store.ListEntitiesByTenant (stub)", zap.String("tenant_id", tenantID))
	// TODO: SELECT ... FROM legal_entities WHERE tenant_id = $1
	return []*domain.LegalEntity{}, nil
}

func (s *PgStore) UpdateEntity(ctx context.Context, legalEntityID string, req domain.UpdateEntityRequest) (*domain.LegalEntity, error) {
	s.log.Debug("store.UpdateEntity (stub)", zap.String("legal_entity_id", legalEntityID))
	// TODO: UPDATE legal_entities SET ... WHERE legal_entity_id = $1 RETURNING *
	return nil, fmt.Errorf("store.UpdateEntity: not implemented")
}

// TransitionEntityStatus updates the entity_status column.
// Must be idempotent: applying the same target status is a no-op at DB level
// (UPDATE ... WHERE entity_status != $newStatus ensures zero rows affected on repeat).
func (s *PgStore) TransitionEntityStatus(ctx context.Context, legalEntityID string, newStatus domain.EntityStatus, actorID, correlationID string) error {
	s.log.Debug("store.TransitionEntityStatus (stub)",
		zap.String("legal_entity_id", legalEntityID),
		zap.String("new_status", string(newStatus)),
	)
	// TODO: UPDATE legal_entities
	//         SET entity_status = $1
	//       WHERE legal_entity_id = $2
	//         AND entity_status   != $1   -- idempotency guard
	return nil
}

func (s *PgStore) GetEntityStatus(ctx context.Context, legalEntityID string) (*domain.EntityStatusResponse, error) {
	s.log.Debug("store.GetEntityStatus (stub)", zap.String("legal_entity_id", legalEntityID))
	// TODO: SELECT tenant_id, entity_status FROM legal_entities WHERE legal_entity_id = $1
	return nil, fmt.Errorf("store.GetEntityStatus: not implemented")
}

// ---------------------------------------------------------------------------
// EntityHierarchy
// ---------------------------------------------------------------------------

func (s *PgStore) CreateHierarchy(ctx context.Context, h *domain.EntityHierarchy) error {
	s.log.Debug("store.CreateHierarchy (stub)", zap.String("hierarchy_id", h.HierarchyID))
	// TODO: INSERT INTO entity_hierarchies (...) VALUES (...)
	return nil
}

func (s *PgStore) EndDateHierarchy(ctx context.Context, hierarchyID string, endDate time.Time, actorID, correlationID string) error {
	s.log.Debug("store.EndDateHierarchy (stub)", zap.String("hierarchy_id", hierarchyID))
	// TODO: UPDATE entity_hierarchies SET effective_to = $1 WHERE hierarchy_id = $2 AND effective_to IS NULL
	return nil
}

func (s *PgStore) ListHierarchiesByEntity(ctx context.Context, legalEntityID string) ([]*domain.EntityHierarchy, error) {
	s.log.Debug("store.ListHierarchiesByEntity (stub)", zap.String("legal_entity_id", legalEntityID))
	return []*domain.EntityHierarchy{}, nil
}

// ---------------------------------------------------------------------------
// EntityJurisdictionAssignment
// ---------------------------------------------------------------------------

func (s *PgStore) CreateJurisdictionAssignment(ctx context.Context, a *domain.EntityJurisdictionAssignment) error {
	s.log.Debug("store.CreateJurisdictionAssignment (stub)", zap.String("assignment_id", a.AssignmentID))
	// TODO: INSERT INTO entity_jurisdiction_assignments (...) VALUES (...)
	return nil
}

func (s *PgStore) ListJurisdictionAssignments(ctx context.Context, legalEntityID string) ([]*domain.EntityJurisdictionAssignment, error) {
	s.log.Debug("store.ListJurisdictionAssignments (stub)", zap.String("legal_entity_id", legalEntityID))
	return []*domain.EntityJurisdictionAssignment{}, nil
}

func (s *PgStore) EndDateJurisdictionAssignment(ctx context.Context, assignmentID string, endDate time.Time, actorID, correlationID string) error {
	s.log.Debug("store.EndDateJurisdictionAssignment (stub)", zap.String("assignment_id", assignmentID))
	// TODO: UPDATE entity_jurisdiction_assignments SET effective_to = $1 WHERE assignment_id = $2
	return nil
}

// ---------------------------------------------------------------------------
// DataResidencyPolicy
// ---------------------------------------------------------------------------

func (s *PgStore) CreateResidencyPolicy(ctx context.Context, p *domain.DataResidencyPolicy) error {
	s.log.Debug("store.CreateResidencyPolicy (stub)", zap.String("policy_id", p.DataResidencyPolicyID))
	return nil
}

func (s *PgStore) GetResidencyPolicyByID(ctx context.Context, policyID string) (*domain.DataResidencyPolicy, error) {
	s.log.Debug("store.GetResidencyPolicyByID (stub)", zap.String("policy_id", policyID))
	return nil, fmt.Errorf("store.GetResidencyPolicyByID: not implemented")
}

// ---------------------------------------------------------------------------
// ResidencyRegion — read-only (IaC-managed, per Q1 resolution)
//
// No CreateResidencyRegion, UpdateResidencyRegion, or DeleteResidencyRegion
// methods exist on this store. Regions are provisioned exclusively via IaC
// (Terraform / CDK). Any attempt to write regions through the API is
// architecturally prohibited.
// ---------------------------------------------------------------------------

func (s *PgStore) GetResidencyRegionByID(ctx context.Context, regionID string) (*domain.ResidencyRegion, error) {
	s.log.Debug("store.GetResidencyRegionByID (stub)", zap.String("region_id", regionID))
	// TODO: SELECT ... FROM residency_regions WHERE residency_region_id = $1 AND active_flag = true
	return nil, fmt.Errorf("store.GetResidencyRegionByID: not implemented")
}

func (s *PgStore) ListResidencyRegions(ctx context.Context) ([]*domain.ResidencyRegion, error) {
	s.log.Debug("store.ListResidencyRegions (stub)")
	// TODO: SELECT ... FROM residency_regions WHERE active_flag = true ORDER BY region_code
	return []*domain.ResidencyRegion{}, nil
}

// ---------------------------------------------------------------------------
// TaxIdentityBundle
//
// This table stores the structural header ONLY:
//   legal_entity_id, jurisdiction_id, status, effective_from, effective_to.
// There is NO column for a tax registration number or any tax identifier value.
// Those live exclusively in the Tax Service.
// ---------------------------------------------------------------------------

func (s *PgStore) CreateTaxIdentityBundle(ctx context.Context, b *domain.TaxIdentityBundle) error {
	s.log.Debug("store.CreateTaxIdentityBundle (stub)", zap.String("bundle_id", b.TaxIdentityBundleID))
	// TODO: INSERT INTO tax_identity_bundles
	//         (tax_identity_bundle_id, legal_entity_id, jurisdiction_id, status, effective_from, effective_to)
	//       VALUES ($1, $2, $3, $4, $5, $6)
	return nil
}

func (s *PgStore) GetTaxIdentityBundleByID(ctx context.Context, bundleID string) (*domain.TaxIdentityBundle, error) {
	s.log.Debug("store.GetTaxIdentityBundleByID (stub)", zap.String("bundle_id", bundleID))
	return nil, fmt.Errorf("store.GetTaxIdentityBundleByID: not implemented")
}

func (s *PgStore) ListTaxIdentityBundlesByEntity(ctx context.Context, legalEntityID string) ([]*domain.TaxIdentityBundle, error) {
	s.log.Debug("store.ListTaxIdentityBundlesByEntity (stub)", zap.String("legal_entity_id", legalEntityID))
	return []*domain.TaxIdentityBundle{}, nil
}

func (s *PgStore) TransitionTaxIdentityBundleStatus(ctx context.Context, bundleID string, newStatus domain.TaxIdentityBundleStatus, actorID, correlationID string) error {
	s.log.Debug("store.TransitionTaxIdentityBundleStatus (stub)",
		zap.String("bundle_id", bundleID),
		zap.String("new_status", string(newStatus)),
	)
	// TODO: UPDATE tax_identity_bundles SET status = $1 WHERE tax_identity_bundle_id = $2
	return nil
}
