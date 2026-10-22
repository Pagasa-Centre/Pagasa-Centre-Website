package campadmin

import (
	"context"
	"testing"

	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/billing/invoicing"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
)

type auditSummaryBill struct {
	fakeBill
}

func (a *auditSummaryBill) RemoveCamper(context.Context, string, string, string, int) (camperedit.RemoveCamperSummary, error) {
	return sampleRemoveCamperSummary(), nil
}
func (a *auditSummaryBill) ConvertCamperToDayVisitor(context.Context, string, string, string, int, camperedit.ConvertToDayVisitorRequest) (camperedit.ConvertSummary, error) {
	return sampleConvertSummary(), nil
}
func (a *auditSummaryBill) UpdateDayPassCamper(context.Context, string, string, string, int, camperedit.UpdateDayPassCamperRequest) (camperedit.UpdateDayPassSummary, error) {
	return sampleUpdateDayPassSummary(), nil
}
func (a *auditSummaryBill) UpdateCamperCoach(context.Context, string, string, string, int, camperedit.UpdateCamperCoachRequest) (camperedit.UpdateCamperCoachSummary, error) {
	return sampleUpdateCamperCoachSummary(), nil
}
func (a *auditSummaryBill) EditCamper(context.Context, string, string, string, int, camperedit.EditCamperRequest) (camperedit.EditCamperSummary, error) {
	return sampleEditCamperSummary(), nil
}
func (a *auditSummaryBill) AddCamper(context.Context, string, string, int, camperedit.AddCamperRequest) (camperedit.AddCamperSummary, error) {
	return sampleAddCamperSummary(), nil
}
func (a *auditSummaryBill) DeleteRegistration(context.Context, string, string, int) (invoicing.DeleteSummary, error) {
	return sampleDeleteSummary(), nil
}

func billAllAudit(b *auditSummaryBill) BillingServices {
	return BillingServices{Allocation: b, Invoicing: b, Campers: b, Sweep: b, Sheets: b}
}

func assertLastMetaGolden(t *testing.T, rec *captureRec, name string) {
	t.Helper()
	if len(rec.calls) == 0 {
		t.Fatal("expected audit record")
	}
	meta := rec.calls[len(rec.calls)-1].meta
	testhelper.AssertGoldenMarshal(t, name, meta)
}

func TestAuditMetadata_golden_removeCamper(t *testing.T) {
	rec := &captureRec{}
	reg := &fakeReg{group: &domain.Group{ID: "g1", ContactFirstName: "Ann", ContactLastName: "Lee"}}
	svc := NewService(billAllAudit(&auditSummaryBill{}), reg, nil, nil, rec)
	if _, err := svc.RemoveCamper(context.Background(), "g1", "c1", "actor", 1); err != nil {
		t.Fatal(err)
	}
	assertLastMetaGolden(t, rec, "audit_meta_remove_camper.json")
}

func TestAuditMetadata_golden_convertCamper(t *testing.T) {
	rec := &captureRec{}
	svc := NewService(billAllAudit(&auditSummaryBill{}), nil, nil, nil, rec)
	if _, err := svc.ConvertCamperToDayVisitor(context.Background(), "g1", "c1", "actor", 1, camperedit.ConvertToDayVisitorRequest{}); err != nil {
		t.Fatal(err)
	}
	assertLastMetaGolden(t, rec, "audit_meta_convert_camper.json")
}

func TestAuditMetadata_golden_dayPassUpdated(t *testing.T) {
	rec := &captureRec{}
	svc := NewService(billAllAudit(&auditSummaryBill{}), nil, nil, nil, rec)
	if _, err := svc.UpdateDayPassCamper(context.Background(), "g1", "c1", "actor", 1, camperedit.UpdateDayPassCamperRequest{}); err != nil {
		t.Fatal(err)
	}
	assertLastMetaGolden(t, rec, "audit_meta_day_pass_updated.json")
}

func TestAuditMetadata_golden_coachUpdated(t *testing.T) {
	rec := &captureRec{}
	svc := NewService(billAllAudit(&auditSummaryBill{}), nil, nil, nil, rec)
	if _, err := svc.UpdateCamperCoach(context.Background(), "g1", "c1", "actor", 1, camperedit.UpdateCamperCoachRequest{}); err != nil {
		t.Fatal(err)
	}
	assertLastMetaGolden(t, rec, "audit_meta_coach_updated.json")
}

func TestAuditMetadata_golden_camperEdited(t *testing.T) {
	rec := &captureRec{}
	svc := NewService(billAllAudit(&auditSummaryBill{}), nil, nil, nil, rec)
	if _, err := svc.EditCamper(context.Background(), "g1", "c1", "actor", 1, camperedit.EditCamperRequest{}); err != nil {
		t.Fatal(err)
	}
	assertLastMetaGolden(t, rec, "audit_meta_camper_edited.json")
}

func TestAuditMetadata_golden_camperAdded(t *testing.T) {
	rec := &captureRec{}
	svc := NewService(billAllAudit(&auditSummaryBill{}), nil, nil, nil, rec)
	if _, err := svc.AddCamper(context.Background(), "g1", "actor", 1, camperedit.AddCamperRequest{}); err != nil {
		t.Fatal(err)
	}
	assertLastMetaGolden(t, rec, "audit_meta_camper_added.json")
}

func TestAuditMetadata_golden_registrationDeleted(t *testing.T) {
	rec := &captureRec{}
	svc := NewService(billAllAudit(&auditSummaryBill{}), nil, nil, nil, rec)
	if _, err := svc.DeleteRegistration(context.Background(), "g1", "actor", 1); err != nil {
		t.Fatal(err)
	}
	assertLastMetaGolden(t, rec, "audit_meta_registration_deleted.json")
}
