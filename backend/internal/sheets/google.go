package sheets

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"google.golang.org/api/option"
	sheetsapi "google.golang.org/api/sheets/v4"
)

// GoogleSync is the production Sync implementation, backed by the Google
// Sheets v4 API. Authenticates with a service-account JSON key. Failures are
// logged by the caller but the API surface returns errors so they can be
// observed in tests.
type GoogleSync struct {
	svc            *sheetsapi.Service
	spreadsheetID  string
	pendingTab     string
	paidTab        string
	pendingSheetID int64
	paidSheetID    int64

	// headerOnce guards lazy header insertion (one per tab, per process).
	headerOnce sync.Map // map[string]*sync.Once keyed by tab name
}

// GoogleSyncConfig is the input bundle for NewGoogleSync.
type GoogleSyncConfig struct {
	// ServiceAccountJSON is the raw or base64-encoded contents of the
	// service-account credentials JSON file downloaded from Google Cloud.
	// Either form is accepted — we detect by trying base64 first.
	ServiceAccountJSON string

	// SpreadsheetID is the long ID from the Sheet URL between /d/ and /edit.
	SpreadsheetID string

	// PendingTab and PaidTab are the case-sensitive tab names inside the
	// spreadsheet. Both tabs must already exist (we don't auto-create).
	PendingTab string
	PaidTab    string
}

// NewGoogleSync constructs a GoogleSync, authenticates with the credentials,
// and resolves the two tab IDs (gids) needed for later batch updates.
//
// Returns an error if authentication fails or either tab doesn't exist —
// caller is expected to fall back to NoopSync in that case rather than
// crash-looping.
func NewGoogleSync(ctx context.Context, cfg GoogleSyncConfig) (*GoogleSync, error) {
	credBytes, err := decodeCredentials(cfg.ServiceAccountJSON)
	if err != nil {
		return nil, fmt.Errorf("decode credentials: %w", err)
	}
	svc, err := sheetsapi.NewService(ctx,
		option.WithCredentialsJSON(credBytes),
		option.WithScopes(sheetsapi.SpreadsheetsScope),
	)
	if err != nil {
		return nil, fmt.Errorf("sheets service: %w", err)
	}

	sp, err := svc.Spreadsheets.Get(cfg.SpreadsheetID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("fetch spreadsheet metadata: %w", err)
	}

	g := &GoogleSync{
		svc:           svc,
		spreadsheetID: cfg.SpreadsheetID,
		pendingTab:    cfg.PendingTab,
		paidTab:       cfg.PaidTab,
	}
	pendingFound, paidFound := false, false
	for _, s := range sp.Sheets {
		if s.Properties == nil {
			continue
		}
		switch s.Properties.Title {
		case cfg.PendingTab:
			g.pendingSheetID = s.Properties.SheetId
			pendingFound = true
		case cfg.PaidTab:
			g.paidSheetID = s.Properties.SheetId
			paidFound = true
		}
	}
	if !pendingFound {
		return nil, fmt.Errorf("tab %q not found in spreadsheet", cfg.PendingTab)
	}
	if !paidFound {
		return nil, fmt.Errorf("tab %q not found in spreadsheet", cfg.PaidTab)
	}
	return g, nil
}

// decodeCredentials accepts the credential blob as either raw JSON (starts
// with `{`) or base64-encoded JSON, and returns the raw bytes. Base64 is
// preferred for env vars to avoid newline/quote escaping headaches.
func decodeCredentials(s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, fmt.Errorf("empty credentials")
	}
	if s[0] == '{' {
		return []byte(s), nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// AppendPending appends rows to the Pending tab.
func (g *GoogleSync) AppendPending(ctx context.Context, rows []Row) error {
	if len(rows) == 0 {
		return nil
	}
	return g.appendRows(ctx, g.pendingTab, rows)
}

// AppendPaidAndRemovePending appends rows to the Paid tab, then deletes any
// rows in the Pending tab whose group_id (column A) matches. Order matters:
// if the second step fails for any reason, the row exists in both tabs —
// preferable to losing it.
func (g *GoogleSync) AppendPaidAndRemovePending(ctx context.Context, groupID string, rows []Row) error {
	if err := g.appendRows(ctx, g.paidTab, rows); err != nil {
		return fmt.Errorf("append paid: %w", err)
	}
	if err := g.deleteByGroupID(ctx, groupID, g.pendingTab, g.pendingSheetID); err != nil {
		// Logged but not fatal — Paid row is in, just means there's a
		// stale Pending row the leaders may see briefly.
		log.Printf("sheets: delete pending for group %s failed: %v", groupID, err)
	}
	return nil
}

// UpdateContactByGroupID rewrites the four contact_* columns (G:J) for every
// row whose group_id (column A) matches, across both tabs. The Paid tab is the
// usual target once a deposit is in, but we also scan Pending so a correction
// made before payment isn't lost.
func (g *GoogleSync) UpdateContactByGroupID(ctx context.Context, groupID string, c ContactUpdate) error {
	for _, tab := range []string{g.paidTab, g.pendingTab} {
		if err := g.updateContactInTab(ctx, tab, groupID, c); err != nil {
			return fmt.Errorf("update contact in %s: %w", tab, err)
		}
	}
	return nil
}

// updateContactInTab finds matching group rows in one tab and rewrites their
// contact columns. Columns G:J map to contact_first_name, contact_last_name,
// contact_email, contact_phone (positions 7-10, matching Headers above).
func (g *GoogleSync) updateContactInTab(ctx context.Context, tab, groupID string, c ContactUpdate) error {
	resp, err := g.svc.Spreadsheets.Values.
		Get(g.spreadsheetID, tab+"!A:A").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("read column A: %w", err)
	}

	var data []*sheetsapi.ValueRange
	for i, row := range resp.Values {
		if len(row) == 0 {
			continue
		}
		if v, ok := row[0].(string); ok && v == groupID {
			rowNum := i + 1 // sheet rows are 1-based
			data = append(data, &sheetsapi.ValueRange{
				Range:  fmt.Sprintf("%s!G%d:J%d", tab, rowNum, rowNum),
				Values: [][]any{{c.FirstName, c.LastName, c.Email, c.Phone}},
			})
		}
	}
	if len(data) == 0 {
		return nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err = g.svc.Spreadsheets.Values.
		BatchUpdate(g.spreadsheetID, &sheetsapi.BatchUpdateValuesRequest{
			ValueInputOption: "USER_ENTERED",
			Data:             data,
		}).
		Context(ctxWithTimeout).
		Do()
	if err != nil {
		return fmt.Errorf("batch update contact: %w", err)
	}
	return nil
}

// appendRows lazily writes headers (if the tab is empty) and then appends
// the row values. Uses USER_ENTERED so Sheet auto-parses dates/numbers.
func (g *GoogleSync) appendRows(ctx context.Context, tab string, rows []Row) error {
	if err := g.ensureHeaders(ctx, tab); err != nil {
		return fmt.Errorf("ensure headers: %w", err)
	}
	values := make([][]any, 0, len(rows))
	for _, r := range rows {
		values = append(values, r.Values())
	}
	_, err := g.svc.Spreadsheets.Values.
		Append(g.spreadsheetID, tab+"!A:A", &sheetsapi.ValueRange{Values: values}).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("append %s: %w", tab, err)
	}
	return nil
}

// ensureHeaders writes the header row to `tab` if A1 is empty. Each tab is
// only checked once per process (cheap sync.Once), so this adds at most one
// extra API call per tab per restart.
func (g *GoogleSync) ensureHeaders(ctx context.Context, tab string) error {
	onceI, _ := g.headerOnce.LoadOrStore(tab, &sync.Once{})
	once := onceI.(*sync.Once)
	var err error
	once.Do(func() {
		resp, getErr := g.svc.Spreadsheets.Values.
			Get(g.spreadsheetID, tab+"!A1").
			Context(ctx).
			Do()
		if getErr != nil {
			err = fmt.Errorf("get A1: %w", getErr)
			return
		}
		if len(resp.Values) > 0 && len(resp.Values[0]) > 0 {
			return // headers already present
		}
		_, updateErr := g.svc.Spreadsheets.Values.
			Update(g.spreadsheetID, tab+"!A1", &sheetsapi.ValueRange{Values: [][]any{Headers}}).
			ValueInputOption("USER_ENTERED").
			Context(ctx).
			Do()
		if updateErr != nil {
			err = fmt.Errorf("write headers: %w", updateErr)
		}
	})
	return err
}

// RemoveByGroupID deletes every row whose group_id (column A) matches from
// both the Paid and Pending tabs.
func (g *GoogleSync) RemoveByGroupID(ctx context.Context, groupID string) error {
	for _, spec := range []struct {
		tab     string
		sheetID int64
	}{
		{g.paidTab, g.paidSheetID},
		{g.pendingTab, g.pendingSheetID},
	} {
		if err := g.deleteByGroupID(ctx, groupID, spec.tab, spec.sheetID); err != nil {
			return fmt.Errorf("delete from %s: %w", spec.tab, err)
		}
	}
	return nil
}

// deleteByGroupID reads column A of tab, finds every row matching groupID,
// then issues a single batchUpdate with DeleteDimension requests sorted
// descending by row index (so earlier indices remain valid after each delete).
func (g *GoogleSync) deleteByGroupID(ctx context.Context, groupID, tab string, sheetID int64) error {
	resp, err := g.svc.Spreadsheets.Values.
		Get(g.spreadsheetID, tab+"!A:A").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("read column A: %w", err)
	}

	// Collect 0-based row indices (sheet API uses 0-based, with header at 0).
	var indices []int64
	for i, row := range resp.Values {
		if len(row) == 0 {
			continue
		}
		if v, ok := row[0].(string); ok && v == groupID {
			indices = append(indices, int64(i))
		}
	}
	if len(indices) == 0 {
		return nil
	}

	// Delete bottom-up so earlier indices don't shift.
	sort.Slice(indices, func(i, j int) bool { return indices[i] > indices[j] })

	requests := make([]*sheetsapi.Request, 0, len(indices))
	for _, idx := range indices {
		requests = append(requests, &sheetsapi.Request{
			DeleteDimension: &sheetsapi.DeleteDimensionRequest{
				Range: &sheetsapi.DimensionRange{
					SheetId:    sheetID,
					Dimension:  "ROWS",
					StartIndex: idx,
					EndIndex:   idx + 1,
				},
			},
		})
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err = g.svc.Spreadsheets.BatchUpdate(g.spreadsheetID, &sheetsapi.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}).Context(ctxWithTimeout).Do()
	if err != nil {
		return fmt.Errorf("batch delete: %w", err)
	}
	return nil
}
