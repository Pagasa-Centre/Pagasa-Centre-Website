#!/usr/bin/env python3
"""Migrate monolithic billing integration tests to split packages."""
from __future__ import annotations

import re
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BILLING = ROOT / "backend/internal/billing"

SKIP_TESTS = {
    "TestDayPassLine",
    "TestCoachLine",
    "TestSendAccommodationChangedNotice",
}

INVOICING_FROM_CAMPER = {
    "TestSendInvoice_billsOutstandingDeposit",
    "TestMarkBalancePaid_settlesOutstandingDeposits",
    "TestDepositOwed_clearedWithoutPaymentIsNotRecordedAsPaid",
}

SKIP_SYMBOLS = {
    "recordingMailer",
    "stubStripe",
    "creditCall",
    "refundInvCall",
    "failingSheets",
    "strPtr",
}


def git_show(path: str) -> str:
    return subprocess.check_output(
        ["git", "show", f"HEAD:{path}"], cwd=ROOT, text=True
    )


def extract_top_level(src: str) -> list[tuple[str, str]]:
    out: list[tuple[str, str]] = []
    for m in re.finditer(
        r"(?m)^(func (?:\([^)]+\) )?(Test\w+|\w+)|type (\w+))",
        src,
    ):
        name = m.group(2) or m.group(3)
        start = m.start()
        brace = src.find("{", m.end() - 1)
        if brace < 0:
            continue
        depth = 0
        j = brace
        while j < len(src):
            if src[j] == "{":
                depth += 1
            elif src[j] == "}":
                depth -= 1
                if depth == 0:
                    j += 1
                    break
            j += 1
        out.append((name, src[start:j].strip() + "\n"))
    return out


def route(name: str, source: str) -> str:
    if source == "camper_edit":
        return "invoicing" if name in INVOICING_FROM_CAMPER else "camperedit"
    if name.startswith("TestAllocate_") or name.startswith("TestConfirmFree_"):
        return "allocation"
    if name in ("TestHandleInvoicePaid_emailsFamily", "TestHandleCoachInvoicePaid"):
        return "reconcile"
    if name.startswith("TestResyncSheet_"):
        return "sheetsync"
    if (
        name.startswith("TestRemoveCamper_")
        or name.startswith("TestConvertCamper")
        or name.startswith("TestUpdateDayPassCamper_")
        or name.startswith("TestUpdateCamperCoach_")
    ):
        return "camperedit"
    if name.startswith(("TestResolvePriceID_", "TestValidateAllocatedUnit_", "TestChildUnder3")):
        return "pricing"
    if name.startswith(
        (
            "TestBalanceInvoiceItems",
            "TestCoachEligibleCount",
            "TestOffPreferenceChanges",
        )
    ):
        return "lineitems"
    return "invoicing"


def split_go_args(s: str) -> list[str]:
    parts: list[str] = []
    depth = 0
    cur: list[str] = []
    for ch in s:
        if ch == "{" or ch == "(":
            depth += 1
        elif ch == "}" or ch == ")":
            depth -= 1
        if ch == "," and depth == 0:
            parts.append("".join(cur).strip())
            cur = []
            continue
        cur.append(ch)
    if cur:
        parts.append("".join(cur).strip())
    return parts


def replace_new_service(body: str) -> str:
    body = body.replace("NewStripeBilling()", "&billingtest.StubStripe{}")
    body = re.sub(r"repo := storage\.NewRepository\(pool\)\n\t", "", body)

    pat = re.compile(r"\w+ := NewService\(")
    out: list[str] = []
    i = 0
    while True:
        m = pat.search(body, i)
        if not m:
            out.append(body[i:])
            break
        out.append(body[i : m.start()])
        j = m.end()
        depth = 1
        while j < len(body) and depth:
            if body[j] == "(":
                depth += 1
            elif body[j] == ")":
                depth -= 1
            j += 1
        args = split_go_args(body[m.end() : j - 1])
        while len(args) < 5:
            args.append("nil")
        _, stripe, mail, sheet, cfg = args[:5]
        if not cfg.startswith("billing."):
            cfg = cfg.replace("Config{", "billing.Config{", 1)
        k = j
        while k < len(body) and body[k] in " \t\n":
            k += 1
        if k < len(body) and body[k] == ".":
            k += 1
            while k < len(body) and body[k] in " \t\n":
                k += 1
        prices_expr = None
        if body.startswith("WithPriceLookup(", k):
            pe_start = k + len("WithPriceLookup(")
            pe = pe_start
            depth = 1
            while pe < len(body) and depth:
                if body[pe] == "(":
                    depth += 1
                elif body[pe] == ")":
                    depth -= 1
                pe += 1
            prices_expr = body[pe_start : pe - 1].strip()
            j = pe
        newenv = "billingtest.NewEnvWithPrices" if prices_expr else "billingtest.NewEnv"
        if cfg == "nil":
            cfg_line = "cfg := billing.Config{}"
        else:
            cfg_line = f"cfg := {cfg}"
        if prices_expr:
            repl = (
                f"{cfg_line}\n\t"
                f"env := {newenv}(t, pool, {stripe}, {mail}, {sheet}, cfg, {prices_expr})\n\t"
                f"repo := env.Repo"
            )
        else:
            repl = (
                f"{cfg_line}\n\t"
                f"env := {newenv}(t, pool, {stripe}, {mail}, {sheet}, cfg)\n\t"
                f"repo := env.Repo"
            )
        out.append(repl)
        i = j
    return "".join(out)


def fix_repo_order(body: str) -> str:
    """Ensure env exists before first repo use in integration tests."""
    if "billingtest.NewEnv" not in body or "repo.UpdateAccommodationStripePrice" not in body:
        return body
    if body.find("env := billingtest.NewEnv") < body.find("repo.UpdateAccommodationStripePrice"):
        return body
    lines = body.split("\n")
    env_idx = next(i for i, l in enumerate(lines) if "env := billingtest.NewEnv" in l)
    cfg_idx = env_idx - 1 if env_idx > 0 and "cfg :=" in lines[env_idx - 1] else env_idx
    repo_idx = next(i for i, l in enumerate(lines) if "repo := env.Repo" in l)
    block = lines[cfg_idx : repo_idx + 1]
    rest = lines[:cfg_idx] + lines[repo_idx + 1 :]
    insert_at = next(i for i, l in enumerate(rest) if "testhelper.MaybePool" in l) + 2
    return "\n".join(rest[:insert_at] + block + rest[insert_at:])


def xform(body: str) -> str:
    body = replace_new_service(body)
    reps = [
        (r"\bstrPtr\(", "strPtr("),
        (r"\bInvoiceLine\b", "billing.InvoiceLine"),
        (r"\bInvoiceResult\b", "billing.InvoiceResult"),
        (r"\bbalanceInvoiceItems\(", "billing.BalanceInvoiceItems("),
        (r"\bcoachEligibleCount\(", "billing.CoachEligibleCount("),
        (r"\boffPreferenceChanges\(", "billing.OffPreferenceChanges("),
        (r"&recordingMailer\b", "&billingtest.RecordingMailer"),
        (r"mailer\.accomChanged", "mailer.AccomChanged"),
        (r"mailer\.balancePaid", "mailer.BalancePaid"),
        (r"mailer\.bookingUpdated", "mailer.BookingUpdated"),
        (r"mailer\.sponsorships", "mailer.Sponsorships"),
        (r"&stubStripe\b", "&billingtest.StubStripe"),
        (r"\brefundInv:", "RefundInv:"),
        (r"\bcreateInvoice:", "CreateInv:"),
        (r"\bgetInvoice:", "GetInvoiceFn:"),
        (r"\bensureCust:", "EnsureCust:"),
        (r"\bvoidInv:", "VoidInv:"),
        (r"voidInvIdem:", "VoidInvIdem:"),
        (r"stub\.voidInvIdem", "stub.VoidInvIdem"),
        (r"\bstorage\.NewRepository\(pool\)", "env.Repo"),
        (r"\bcfg := Config\{", "cfg := billing.Config{"),
        (r"stub\.voidInv\b", "stub.VoidInv"),
        (r"stub\.createInvoice", "stub.CreateInv"),
        (r"stub\.getInvoice", "stub.GetInvoiceFn"),
        (r"stub\.ensureCust", "stub.EnsureCust"),
        (r"stub\.refundInv\b", "stub.RefundInv"),
        (r"stub\.refundInvCall", "stub.RefundInvCall"),
        (r"stub\.creditBalance", "stub.CreditBalance"),
        (r"stub\.creditCalls", "stub.CreditCalls"),
        (r"failingSheets\b", "billingtest.FailingSheets"),
        (r"&recordingSheets\b", "&billingtest.RecordingSheets"),
        (r"rec\.removeCalls", "rec.RemoveCalls"),
        (r"rec\.appendCalls", "rec.AppendCalls"),
        (r"rec\.pendingCalls", "rec.PendingCalls"),
        (r"rec\.lastGroupID", "rec.LastGroupID"),
        (r"rec\.lastRows", "rec.LastRows"),
        (r"svc\.Allocate\(", "env.Allocation.Allocate("),
        (r"svc\.ConfirmFree\(", "env.Allocation.ConfirmFree("),
        (r"svc\.SendInvoice\(", "env.Invoicing.SendInvoice("),
        (r"svc\.SendInvoicesBulk\(", "env.Invoicing.SendInvoicesBulk("),
        (r"svc\.SendCoachInvoice\(", "env.Invoicing.SendCoachInvoice("),
        (r"svc\.WaiveCoachFee\(", "env.Invoicing.WaiveCoachFee("),
        (r"svc\.UnwaiveCoachFee\(", "env.Invoicing.UnwaiveCoachFee("),
        (r"svc\.MarkBalancePaidManually\(", "env.Invoicing.MarkBalancePaidManually("),
        (r"svc\.DeleteRegistration\(", "env.Invoicing.DeleteRegistration("),
        (r"svc\.RemoveCamper\(", "env.Campers.RemoveCamper("),
        (r"svc\.ConvertCamperToDayVisitor\(", "env.Campers.ConvertCamperToDayVisitor("),
        (r"svc\.UpdateDayPassCamper\(", "env.Campers.UpdateDayPassCamper("),
        (r"svc\.UpdateCamperCoach\(", "env.Campers.UpdateCamperCoach("),
        (r"svc\.EditCamper\(", "env.Campers.EditCamper("),
        (r"svc\.AddCamper\(", "env.Campers.AddCamper("),
        (r"svc\.MakeMainContact\(", "env.Campers.MakeMainContact("),
        (r"svc\.WaiveCamperDeposit\(", "env.Campers.WaiveCamperDeposit("),
        (r"svc\.HandleInvoicePaid\(", "env.Reconcile.HandleInvoicePaid("),
        (r"svc\.HandleCoachInvoicePaid\(", "env.Reconcile.HandleCoachInvoicePaid("),
        (r"svc\.ResyncSheet\(", "env.Sheets.ResyncSheet("),
        (r"svcInv\.SendInvoice\(", "env.Invoicing.SendInvoice("),
        (r"svc\.resolvePriceID\(", "env.Pricer.ResolvePriceID("),
        (r"svc\.validateAllocatedUnit\(", "env.Pricer.ValidateAllocatedUnit("),
        (r"\bAllocateRequest\b", "allocation.AllocateRequest"),
        (r"\bAllocateCamper\b", "allocation.AllocateCamper"),
        (r"\bDeleteSummary\b", "invoicing.DeleteSummary"),
        (r"\bRemoveCamperSummary\b", "camperedit.RemoveCamperSummary"),
        (r"\bConvertSummary\b", "camperedit.ConvertSummary"),
        (r"\bConvertToDayVisitorRequest\b", "camperedit.ConvertToDayVisitorRequest"),
        (r"\bUpdateDayPassSummary\b", "camperedit.UpdateDayPassSummary"),
        (r"\bUpdateDayPassCamperRequest\b", "camperedit.UpdateDayPassCamperRequest"),
        (r"\bUpdateCamperCoachSummary\b", "camperedit.UpdateCamperCoachSummary"),
        (r"\bUpdateCamperCoachRequest\b", "camperedit.UpdateCamperCoachRequest"),
        (r"\bEditCamperRequest\b", "camperedit.EditCamperRequest"),
        (r"\bEditCamperSummary\b", "camperedit.EditCamperSummary"),
        (r"\bAddCamperRequest\b", "camperedit.AddCamperRequest"),
        (r"\bAddCamperSummary\b", "camperedit.AddCamperSummary"),
        (r"\bstrVal\(", "billing.StrVal("),
    ]
    for a, b in reps:
        body = re.sub(a, b, body)
    body = fix_repo_order(body)
    body = re.sub(
        r"(\tctx := context\.Background\(\)\n)\trepo := env\.Repo\n\n",
        r"\1",
        body,
    )
    body = body.replace(".keepPence", ".KeepPence")
    body = body.replace(".amountPence", ".AmountPence")
    body = body.replace(".idempotencyKey", ".IdempotencyKey")
    body = body.replace("*stubStripe", "*billingtest.StubStripe")
    body = body.replace(
        "repo.SetCoachInvoiceMeta",
        "invstorage.NewRepository(pool).SetCoachInvoiceMeta",
    )
    body = body.replace("stripe.creditCalls", "stripe.CreditCalls")
    return body


HEADERS = {
    "allocation": """package allocation_test

import (
\t"context"
\t"errors"
\t"testing"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/billing/allocation"
\t"pagasacentre/backend/internal/billing/billingtest"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/internal/testhelper"
\t"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func strPtr(s string) *string { return &s }
""",
    "invoicing": """package invoicing_test

import (
\t"context"
\t"errors"
\t"strings"
\t"testing"
\t"time"

\t"github.com/jackc/pgx/v5/pgxpool"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/billing/allocation"
\t"pagasacentre/backend/internal/billing/billingtest"
\t"pagasacentre/backend/internal/billing/camperedit"
\t"pagasacentre/backend/internal/billing/invoicing"
\tinvstorage "pagasacentre/backend/internal/billing/invoicing/storage"
\t"pagasacentre/backend/internal/email"
\t"pagasacentre/backend/internal/registration"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/internal/sheets"
\t"pagasacentre/backend/internal/testhelper"
\t"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func strPtr(s string) *string { return &s }
""",
    "camperedit": """package camperedit_test

import (
\t"context"
\t"errors"
\t"strings"
\t"testing"

\t"github.com/jackc/pgx/v5/pgxpool"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/billing/billingtest"
\t"pagasacentre/backend/internal/billing/camperedit"
\t"pagasacentre/backend/internal/email"
\t"pagasacentre/backend/internal/registration"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/internal/testhelper"
\t"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func strPtr(s string) *string { return &s }
""",
    "reconcile": """package reconcile_test

import (
\t"context"
\t"testing"
\t"time"

\t"github.com/jackc/pgx/v5/pgxpool"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/billing/billingtest"
\tinvstorage "pagasacentre/backend/internal/billing/invoicing/storage"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/internal/testhelper"
)
""",
    "sheetsync": """package sheetsync_test

import (
\t"context"
\t"testing"

\t"github.com/jackc/pgx/v5/pgxpool"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/billing/billingtest"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/internal/testhelper"
)
""",
    "pricing": """package billing_test

import (
\t"context"
\t"testing"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/billing/billingtest"
\t"pagasacentre/backend/internal/registration"
\t"pagasacentre/backend/internal/testhelper"
)
""",
    "lineitems": """package billing_test

import (
\t"testing"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/registration/domain"
)

func strPtr(s string) *string { return &s }
""",
}

HELPERS = {
    "insertThreeCamperGroup": "invoicing",
    "insertCoachGroup": "invoicing",
    "insertTwoCoachGroup": "invoicing",
    "insertEditableGroup": "camperedit",
    "editFor": "camperedit",
    "camperByID": "camperedit",
    "newFullWeekCamper": "camperedit",
    "fakePrices": "camperedit",
    "mutablePrices": "camperedit",
    "insertDayPassGroup": "sheetsync",
    "validUpdateDayPassRequest": "camperedit",
    "validConvertRequest": "camperedit",
    "countTierAllocations": "camperedit",
}


def fix_one_stub_env(text: str) -> str:
    m = re.search(
        r"(\tcfg := billing\.Config\{[^\n]*\}\n)"
        r"\tenv := billingtest\.NewEnv\(t, pool, stub,([^\n]+)\n"
        r"\trepo := env\.Repo\n"
        r"(.*?)"
        r"(\t(?:var captured[^\n]*\n(?:\tvar [^\n]+\n)*)?\tstub := &billingtest\.StubStripe\{.*?\n\t\}\n)",
        text,
        re.S,
    )
    if not m:
        return text
    header, env_tail, middle, stub = m.groups()
    env_line = f"\tenv := billingtest.NewEnv(t, pool, stub,{env_tail}\n\trepo := env.Repo\n"
    return text[: m.start()] + header + stub + env_line + middle + text[m.end() :]


def fix_repo_with_pricer(content: str) -> str:
    return re.sub(
        r"env := billingtest\.NewEnv\([^\n]+\)\n\trepo := env\.Repo\.\n\s*WithPriceLookup\(([^)]+)\)",
        lambda m: f"env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{{}}, &billingtest.RecordingMailer{{}}, nil, billing.Config{{}}, {m.group(1)})\n\trepo := env.Repo",
        content,
    )


def fix_settle_map(content: str) -> str:
    content = content.replace(
        "settle := map[string]func(*Service, string) error{",
        "settle := map[string]func(string) error{",
    )
    content = content.replace("func(svc *Service, groupID string)", "func(groupID string)")
    content = content.replace("run(svc, groupID)", "run(groupID)")
    content = content.replace(
        "clear := map[string]func(*Service, string, string) error{",
        "clear := map[string]func(string, string) error{",
    )
    content = content.replace("func(svc *Service, groupID, camperID string)", "func(groupID, camperID string)")
    content = content.replace("run(svc, groupID, otherID)", "run(groupID, otherID)")
    content = content.replace("run(svc, groupID, camperID)", "run(groupID, camperID)")
    # settle map closures need env in outer scope
    content = re.sub(
        r"func TestMarkBalancePaid_settlesOutstandingDeposits\(t \*testing\.T\) \{\n\tpool := testhelper\.MaybePool\(t\)\n\tctx := context\.Background\(\)\n\tsettle :=",
        "func TestMarkBalancePaid_settlesOutstandingDeposits(t *testing.T) {\n\tpool := testhelper.MaybePool(t)\n\tctx := context.Background()\n\tcfg := billing.Config{}\n\tenv := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)\n\tsettle :=",
        content,
        count=1,
    )
    content = re.sub(
        r"func TestDepositOwed_clearedWithoutPaymentIsNotRecordedAsPaid\(t \*testing\.T\) \{\n\tpool := testhelper\.MaybePool\(t\)\n\tctx := context\.Background\(\)\n\tneedsCatering := false\n\n\tclear :=",
        "func TestDepositOwed_clearedWithoutPaymentIsNotRecordedAsPaid(t *testing.T) {\n\tpool := testhelper.MaybePool(t)\n\tctx := context.Background()\n\tneedsCatering := false\n\tcfg := billing.Config{}\n\tenv := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, fakePrices{depositPence: 5000})\n\n\tclear :=",
        content,
        count=1,
    )
    return content


def drop_unused_repo(content: str) -> str:
    chunks = re.split(r"(?m)(?=^func )", content)
    out: list[str] = []
    for chunk in chunks:
        if "repo := env.Repo" in chunk and "repo." not in chunk.replace("repo := env.Repo", ""):
            chunk = re.sub(r"\trepo := env\.Repo\n", "", chunk)
        out.append(chunk)
    return "".join(out)


def write(path: Path, content: str) -> None:
    content = fix_settle_map(content)
    content = fix_repo_with_pricer(content)
    content = drop_unused_repo(content)
    path.parent.mkdir(parents=True, exist_ok=True)
    if not content.endswith("\n"):
        content += "\n"
    path.write_text(content)


def main() -> None:
    buckets: dict[str, list[str]] = {k: [] for k in HEADERS}
    helpers: dict[str, list[str]] = {k: [] for k in HEADERS}

    for src, key in (
        (git_show("backend/internal/billing/service_test.go"), "service"),
        (git_show("backend/internal/billing/camper_edit_test.go"), "camper_edit"),
    ):
        for name, body in extract_top_level(src):
            if name.startswith("Test"):
                if name in SKIP_TESTS:
                    continue
                dest = route(name, key)
                buckets[dest].append(xform(body))
            elif name == "GetPrice" and ("fakePrices" in body or "mutablePrices" in body):
                helpers["camperedit"].append(xform(body))
                helpers["invoicing"].append(xform(body))
            elif name in HELPERS:
                helpers[HELPERS[name]].append(xform(body))

    write(BILLING / "pricing_test.go", HEADERS["pricing"] + "\n" + "\n\n".join(buckets["pricing"]))
    write(
        BILLING / "lineitems_test.go",
        HEADERS["lineitems"] + "\n" + "\n\n".join(buckets["lineitems"]),
    )

    if helpers["invoicing"]:
        coach_ins = [h for h in helpers["invoicing"] if h.startswith("func insertCoachGroup")]
        if coach_ins:
            helpers["reconcile"] = coach_ins + helpers.get("reconcile", [])
        three_ins = [h for h in helpers["invoicing"] if h.startswith("func insertThreeCamperGroup")]
        if three_ins:
            helpers["camperedit"] = three_ins + helpers.get("camperedit", [])
        two_coach = [h for h in helpers["invoicing"] if h.startswith("func insertTwoCoachGroup")]
        if two_coach:
            helpers["camperedit"] = two_coach + helpers.get("camperedit", [])
        coach_g = [h for h in helpers["invoicing"] if h.startswith("func insertCoachGroup")]
        if coach_g:
            helpers["camperedit"] = coach_g + helpers.get("camperedit", [])
        day_g = [h for h in helpers["sheetsync"] if h.startswith("func insertDayPassGroup")]
        if day_g:
            helpers["camperedit"] = day_g + helpers.get("camperedit", [])
        for fn in ("insertEditableGroup", "editFor", "camperByID", "fakePrices", "mutablePrices"):
            h = [x for x in helpers.get("camperedit", []) if x.startswith(f"func {fn}") or x.startswith(f"type {fn}")]
            if h:
                helpers["invoicing"] = h + helpers.get("invoicing", [])

    for pkg in ("allocation", "invoicing", "camperedit", "reconcile", "sheetsync"):
        parts = helpers.get(pkg, []) + buckets.get(pkg, [])
        write(
            BILLING / pkg / "service_test.go",
            HEADERS[pkg] + "\n" + "\n\n".join(parts),
        )

    print("regenerated billing tests")


if __name__ == "__main__":
    main()
