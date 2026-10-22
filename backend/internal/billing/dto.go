package billing

// BulkInvoiceRequest is POST /camp-admin/registrations/invoice-bulk.
type BulkInvoiceRequest struct {
	GroupIDs []string `json:"group_ids"`
}

// VersionedBody is embedded by POST handlers that accept optional optimistic concurrency.
type VersionedBody struct {
	ExpectedVersion *int `json:"expected_version,omitempty"`
}
