#!/usr/bin/env python3
"""Post-process split billing tests: external packages, env order, imports."""
from __future__ import annotations

import re
from pathlib import Path

BACKEND = Path(__file__).resolve().parents[1]
BILLING = BACKEND / "internal/billing"

PKG_MAP = {
    "allocation/service_test.go": ("allocation_test", "allocation"),
    "invoicing/service_test.go": ("invoicing_test", "invoicing"),
    "camperedit/service_test.go": ("camperedit_test", "camperedit"),
    "reconcile/service_test.go": ("reconcile_test", "reconcile"),
    "sheetsync/service_test.go": ("sheetsync_test", "sheetsync"),
    "pricing_test.go": ("billing_test", "billing"),
}


def fix_env_order(text: str) -> str:
    pat = re.compile(
        r"\trepo := env\.Repo\n"
        r"(\t(?:cfg := billing\.Config\{[^\n]*\}\n)?)"
        r"\tenv := billingtest\.NewEnv\(t, pool, ([^\n]+)\)\n",
        re.MULTILINE,
    )

    def repl(m: re.Match[str]) -> str:
        cfg_line = m.group(1) or "\tcfg := billing.Config{}\n"
        rest = m.group(2)
        return cfg_line + f"\tenv := billingtest.NewEnv(t, pool, {rest})\n\trepo := env.Repo\n"

    return pat.sub(repl, text)


def convert_external(rel: str, text: str) -> str:
    test_pkg, prod_pkg = PKG_MAP[rel]
    text = re.sub(r"^package \w+", f"package {test_pkg}", text, count=1, flags=re.M)
    if f'"{prod_pkg}"' not in text and prod_pkg != "billing":
        # insert import after billingtest line
        imp = f'\t"pagasacentre/backend/internal/billing/{prod_pkg}"\n'
        if prod_pkg == "billing":
            imp = '\t"pagasacentre/backend/internal/billing"\n'
        text = text.replace(
            '\t"pagasacentre/backend/internal/billing/billingtest"\n',
            f'\t"pagasacentre/backend/internal/billing/billingtest"\n{imp}',
            1,
        )
    if prod_pkg == "billing":
        if '"pagasacentre/backend/internal/billing"' not in text:
            text = text.replace(
                "import (",
                'import (\n\t"pagasacentre/backend/internal/billing"',
                1,
            )
        text = text.replace("billing.Config", "billing.Config")
    # qualification for types when in external package
    if prod_pkg == "allocation":
        text = re.sub(r"\ballocation\.Allocate", "allocation.Allocate", text)
    if prod_pkg == "invoicing":
        text = text.replace("invSvc.sendAccommodationChangedNotice", "invSvc.sendAccommodationChangedNotice")
    return text


def fix_invoicing_accom_test(text: str) -> str:
    if "TestSendAccommodationChangedNotice" not in text:
        return text
    old = textwrap_old = None
    block = """func TestSendAccommodationChangedNotice(t *testing.T) {
	mailer := &billingtest.RecordingMailer{}
	invSvc := invoicing.NewService(nil, nil, &billingtest.StubStripe{}, mailer, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})

	changes := []email.AccommodationChange{{
		CamperName:   "Alex Test",
		FirstChoice:  "Lodge",
		SecondChoice: "Cabin",
		Allocated:    "Tent",
		TentGuidance: true,
	}}
	invSvc.sendAccommodationChangedNotice(context.Background(), "family@example.com", "Sam", changes, true)

	if len(mailer.AccomChanged) != 1 {
		t.Fatalf("expected 1 accommodation-changed email, got %d", len(mailer.AccomChanged))
	}
	if !mailer.AccomChanged[0].AwaitingPayment {
		t.Fatal("expected AwaitingPayment true")
	}
	if mailer.AccomChanged[0].ToEmail != "family@example.com" {
		t.Fatalf("to = %q", mailer.AccomChanged[0].ToEmail)
	}

	before := len(mailer.AccomChanged)
	invSvc.sendAccommodationChangedNotice(context.Background(), "family@example.com", "Sam", nil, true)
	if len(mailer.AccomChanged) != before {
		t.Fatal("should not send when changes empty")
	}
}
"""
    text = re.sub(
        r"func TestSendAccommodationChangedNotice\(t \*testing\.T\) \{.*?\n\}\n",
        block + "\n",
        text,
        count=1,
        flags=re.DOTALL,
    )
    if "invoicing.NewService" in text and '"pagasacentre/backend/internal/billing/invoicing"' not in text:
        pass  # added by convert
    return text


def main() -> None:
    for rel in PKG_MAP:
        p = BILLING / rel
        if not p.exists():
            continue
        text = p.read_text()
        text = fix_env_order(text)
        text = convert_external(rel, text)
        if rel == "invoicing/service_test.go":
            text = fix_invoicing_accom_test(text)
        p.write_text(text)

    # lineitems: ensure package billing, no billingtest
    li = BILLING / "lineitems_test.go"
    if li.exists():
        t = li.read_text()
        t = t.replace("billingtestStrPtr", "strPtr")
        if "func strPtr" not in t:
            t = t.rstrip() + "\n\nfunc strPtr(s string) *string { return &s }\n"
        li.write_text(t)

    print("fixed billing tests")


if __name__ == "__main__":
    main()
