package billing

// AllocateCamper is one camper placement from the admin dashboard.
type AllocateCamper struct {
	CamperID                   string `json:"camper_id"`
	AllocatedAccommodationCode string `json:"allocated_accommodation_code"`
	// Optional physical unit within the tier (e.g. caravan_5).
	AllocatedUnitCode string `json:"allocated_unit_code,omitempty"`
	// Optional override; if empty, resolved from tier + age.
	BilledStripePriceID string `json:"billed_stripe_price_id,omitempty"`
}

// AllocateRequest is PUT /admin/registrations/{groupID}/allocation.
type AllocateRequest struct {
	Campers          []AllocateCamper `json:"campers"`
	ExpectedVersion  *int             `json:"expected_version,omitempty"`
}

// BulkInvoiceRequest is POST /admin/registrations/invoice-bulk.
type BulkInvoiceRequest struct {
	GroupIDs []string `json:"group_ids"`
}

// ExtendDueRequest is PATCH /admin/registrations/{groupID}/invoice-due.
type ExtendDueRequest struct {
	DueAt           string `json:"due_at"` // RFC3339
	ExpectedVersion *int   `json:"expected_version,omitempty"`
}

// VersionedBody is embedded by POST handlers that accept optional optimistic concurrency.
type VersionedBody struct {
	ExpectedVersion *int `json:"expected_version,omitempty"`
}
