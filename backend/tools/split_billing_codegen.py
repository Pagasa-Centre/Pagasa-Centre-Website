#!/usr/bin/env python3
"""One-off codegen for billing use-case split. Run from backend/."""
from __future__ import annotations

import re
import textwrap
from pathlib import Path

BACKEND = Path(__file__).resolve().parents[1]
BILLING = BACKEND / "internal/billing"
REG_STORAGE = BACKEND / "internal/registration/storage"


def read(p: Path) -> str:
    return p.read_text()


def write(p: Path, content: str) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(content)
    if not content.endswith("\n"):
        p.write_text(content + "\n")


def storage_header() -> str:
    return textwrap.dedent(
        """\
        package storage

        import (
        \t"context"
        \t"fmt"
        \t"time"

        \t"github.com/jackc/pgx/v5/pgxpool"

        \t"pagasacentre/backend/internal/registration/domain"
        \t"pagasacentre/backend/internal/registration/storage/groupsql"
        \tcommondb "pagasacentre/backend/pkg/commonlibrary/db"
        )

        type Repository struct {
        \tpool *pgxpool.Pool
        }

        func NewRepository(pool *pgxpool.Pool) *Repository {
        \treturn &Repository{pool: pool}
        }

        """
    )


def xform_storage(body: str) -> str:
    body = body.replace("r.getGroupByIDForUpdate", "groupsql.LockGroup")
    body = body.replace("r.stampGroupPool", "groupsql.StampPool")
    return body


def extract_funcs(src: str, names: list[str]) -> str:
    out = []
    for name in names:
        pat = rf"^func (\(s \*Service\) )?{re.escape(name)}\("
        m = re.search(pat, src, re.MULTILINE)
        if not m:
            raise SystemExit(f"missing func {name}")
        start = m.start()
        # find matching brace for top-level func
        i = src.find("{", m.end() - 1)
        depth = 0
        j = i
        while j < len(src):
            if src[j] == "{":
                depth += 1
            elif src[j] == "}":
                depth -= 1
                if depth == 0:
                    j += 1
                    break
            j += 1
        chunk = src[start:j]
        out.append(chunk)
    return "\n\n".join(out)


def xform_service(chunk: str, pkg: str) -> str:
    chunk = chunk.replace("func (s *Service)", "func (s *Service)")
    chunk = re.sub(r"\bs\.repo\.FindGroupByID\b", "s.reads.FindGroupByID", chunk)
    chunk = re.sub(r"\bs\.repo\.CampersForGroup\b", "s.reads.CampersForGroup", chunk)
    chunk = re.sub(r"\bs\.repo\.ListWithBilling\b", "s.reads.ListWithBilling", chunk)
    chunk = re.sub(r"\bmapVersionErr\b", "billing.MapVersionErr", chunk)
    chunk = re.sub(r"\bs\.resolvePriceID\b", "s.pricer.ResolvePriceID", chunk)
    chunk = re.sub(r"\bs\.validateAllocatedUnit\b", "s.pricer.ValidateAllocatedUnit", chunk)
    chunk = re.sub(r"\bs\.accommodationNames\b", "s.pricer.AccommodationNames", chunk)
    chunk = re.sub(r"\bs\.unitNames\b", "s.pricer.UnitNames", chunk)
    chunk = re.sub(r"\bs\.depositAmountPence\b", "s.pricer.DepositAmountPence", chunk)
    chunk = re.sub(r"\bs\.priceFor\b", "s.pricer.PriceFor", chunk)
    chunk = re.sub(r"\bbalanceInvoiceItems\b", "billing.BalanceInvoiceItems", chunk)
    chunk = re.sub(r"\brosterItems\b", "billing.RosterItems", chunk)
    chunk = re.sub(r"\bcoachEligibleCount\b", "billing.CoachEligibleCount", chunk)
    chunk = re.sub(r"\bformatAmountLabel\b", "billing.FormatAmountLabel", chunk)
    chunk = re.sub(r"\boffPreferenceChanges\b", "billing.OffPreferenceChanges", chunk)
    chunk = re.sub(r"\bstrVal\b", "billing.StrVal", chunk)
    chunk = re.sub(r"\bsheetRowsForGroup\b", "billing.SheetRowsForGroup", chunk)
    chunk = re.sub(r"\bInvoiceLine\b", "billing.InvoiceLine", chunk)
    chunk = re.sub(r"\bConfig\b", "billing.Config", chunk)
    chunk = re.sub(r"\bErrCoachPriceNotConfigured\b", "billing.ErrCoachPriceNotConfigured", chunk)
    chunk = re.sub(r"\bErrSponsoredNoCoach\b", "billing.ErrSponsoredNoCoach", chunk)
    chunk = re.sub(r"\bErrCoachInBalance\b", "billing.ErrCoachInBalance", chunk)
    chunk = re.sub(r"\bAllocateRequest\b", "AllocateRequest", chunk)
    chunk = re.sub(r"\bAllocateCamper\b", "AllocateCamper", chunk)
    chunk = re.sub(r"\bExtendDueRequest\b", "ExtendDueRequest", chunk)
    chunk = re.sub(r"\bDeleteSummary\b", "DeleteSummary", chunk)
    chunk = re.sub(r"\bRemoveCamperSummary\b", "RemoveCamperSummary", chunk)
    chunk = re.sub(r"\bConvertSummary\b", "ConvertSummary", chunk)
    chunk = re.sub(r"\bConvertToDayVisitorRequest\b", "ConvertToDayVisitorRequest", chunk)
    chunk = re.sub(r"\bUpdateDayPassSummary\b", "UpdateDayPassSummary", chunk)
    chunk = re.sub(r"\bUpdateDayPassCamperRequest\b", "UpdateDayPassCamperRequest", chunk)
    chunk = re.sub(r"\bUpdateCamperCoachSummary\b", "UpdateCamperCoachSummary", chunk)
    chunk = re.sub(r"\bUpdateCamperCoachRequest\b", "UpdateCamperCoachRequest", chunk)
    chunk = re.sub(r"\bEditCamperRequest\b", "EditCamperRequest", chunk)
    chunk = re.sub(r"\bEditCamperSummary\b", "EditCamperSummary", chunk)
    chunk = re.sub(r"\bAddCamperRequest\b", "AddCamperRequest", chunk)
    chunk = re.sub(r"\bAddCamperSummary\b", "AddCamperSummary", chunk)
    if pkg == "sweep":
        chunk = re.sub(
            r"\bs\.VoidAndRelease\b",
            "s.releaser.VoidAndRelease",
            chunk,
        )
    return chunk


SERVICE_IMPORTS = {
    "allocation": """package allocation

import (
\t"context"
\t"fmt"
\t"log"
\t"strings"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/email"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/pkg/commonlibrary/apperr"
)
""",
    "invoicing": """package invoicing

import (
\t"context"
\t"fmt"
\t"log"
\t"strings"
\t"time"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/email"
\t"pagasacentre/backend/internal/registration"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/internal/sheets"
\t"pagasacentre/backend/pkg/commonlibrary/apperr"
)
""",
    "camperedit": """package camperedit

import (
\t"context"
\t"fmt"
\t"log"
\t"strings"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/email"
\t"pagasacentre/backend/internal/registration"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/internal/sheets"
\t"pagasacentre/backend/pkg/commonlibrary/apperr"
)
""",
    "reconcile": """package reconcile

import (
\t"context"
\t"fmt"
\t"log"
\t"strings"
\t"time"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/email"
\t"pagasacentre/backend/internal/registration/domain"
)
""",
    "sweep": """package sweep

import (
\t"context"
\t"log"
\t"time"

\t"pagasacentre/backend/internal/registration/domain"
)
""",
    "sheetsync": """package sheetsync

import (
\t"context"
\t"fmt"

\t"pagasacentre/backend/internal/billing"
\t"pagasacentre/backend/internal/registration/domain"
\t"pagasacentre/backend/internal/sheets"
\t"pagasacentre/backend/pkg/commonlibrary/apperr"
)
""",
}


def main() -> None:
    mutations = read(REG_STORAGE / "billing_mutations.go")
    camper_mut = read(REG_STORAGE / "camper_mutations.go")
    bill_repo = read(REG_STORAGE / "billing_repository.go")
    svc = read(BILLING / "service.go")
    camper = read(BILLING / "camper_edit.go")

    # --- kernel conflict export ---
    conflict = read(BILLING / "conflict.go")
    conflict = conflict.replace("func staleConflict", "func StaleConflict")
    conflict = conflict.replace("return staleConflict(", "return StaleConflict(")
    conflict = conflict.replace("func mapVersionErr", "func MapVersionErr")
    write(BILLING / "conflict.go", conflict)

    # --- storage: allocation ---
    alloc_body = ""
    for fn in [
        "AllocateGroup",
        "UnallocateGroup",
        "ConfirmFreeMeta",
        "ClearInvoiceAndReleaseMeta",
    ]:
        m = re.search(rf"^func \(r \*Repository\) {fn}\(", mutations, re.MULTILINE)
        if not m:
            raise SystemExit(f"missing {fn}")
        start = m.start()
        nxt = re.search(r"^func \(r \*Repository\)", mutations[m.end() :], re.MULTILINE)
        end = len(mutations) if not nxt else m.end() + nxt.start()
        alloc_body += mutations[start:end] + "\n"
    write(
        BILLING / "allocation/storage/repository.go",
        storage_header() + xform_storage(alloc_body),
    )

    # invoicing storage — extract until DeleteCamperMeta
    inv_fns = [
        "SetInvoiceDetailsMeta",
        "SetCoachInvoiceMeta",
        "MarkCoachFeePaidMeta",
        "WaiveCoachFeeMeta",
        "UnwaiveCoachFeeMeta",
        "DeleteGroupMeta",
        "ExtendInvoiceDueAtMeta",
    ]
    inv_body = ""
    for fn in inv_fns:
        m = re.search(rf"^func \(r \*Repository\) {fn}\(", mutations, re.MULTILINE)
        start = m.start()
        nxt = re.search(r"^func \(r \*Repository\)", mutations[m.end() :], re.MULTILINE)
        end = len(mutations) if not nxt else m.end() + nxt.start()
        inv_body += mutations[start:end] + "\n"
    inv_body += textwrap.dedent(
        """
        func (r *Repository) StampOnly(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
        \treturn groupsql.StampOnly(ctx, r.pool, groupID, meta, tr)
        }

        func (r *Repository) MarkBalancePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
        \treturn groupsql.MarkBalancePaid(ctx, r.pool, groupID, meta, tr)
        }

        func (r *Repository) SetStripeCustomerID(ctx context.Context, groupID, customerID string) error {
        \treturn groupsql.SetStripeCustomerID(ctx, r.pool, groupID, customerID)
        }
        """
    )
    write(BILLING / "invoicing/storage/repository.go", storage_header() + xform_storage(inv_body))

    # camperedit storage
    camp_body = camper_mut + "\n"
    for fn in [
        "DeleteCamperMeta",
        "ConvertCamperToDayPassMeta",
        "UpdateDayPassCamperMeta",
        "UpdateCamperCoachMeta",
    ]:
        m = re.search(rf"^func \(r \*Repository\) {fn}\(", mutations, re.MULTILINE)
        start = m.start()
        nxt = re.search(r"^func \(r \*Repository\)", mutations[m.end() :], re.MULTILINE)
        end = len(mutations) if not nxt else m.end() + nxt.start()
        camp_body += mutations[start:end] + "\n"
    camp_body += textwrap.dedent(
        """
        func (r *Repository) SetStripeCustomerID(ctx context.Context, groupID, customerID string) error {
        \treturn groupsql.SetStripeCustomerID(ctx, r.pool, groupID, customerID)
        }

        func strPtr(s string) *string {
        \tif s == "" {
        \t\treturn nil
        \t}
        \treturn &s
        }
        """
    )
    write(BILLING / "camperedit/storage/repository.go", storage_header() + xform_storage(camp_body))

    # reconcile storage
    recon_body = ""
    for fn in ["FindGroupByStripeInvoiceID", "FindGroupByStripeCoachInvoiceID"]:
        m = re.search(rf"^func \(r \*Repository\) {fn}\(", bill_repo, re.MULTILINE)
        start = m.start()
        nxt = re.search(r"^func \(r \*Repository\)", bill_repo[m.end() :], re.MULTILINE)
        end = len(bill_repo) if not nxt else m.end() + nxt.start()
        chunk = bill_repo[start:end]
        chunk = chunk.replace("r.db(ctx)", "r.pool")
        chunk = chunk.replace("groupSelectCols", "groupsql.GroupSelectCols")
        chunk = chunk.replace("scanGroup", "groupsql.ScanGroup")
        recon_body += chunk + "\n"
    # MarkCoachFeePaidMeta from mutations
    m = re.search(r"^func \(r \*Repository\) MarkCoachFeePaidMeta\(", mutations, re.MULTILINE)
    start = m.start()
    nxt = re.search(r"^func \(r \*Repository\)", mutations[m.end() :], re.MULTILINE)
    end = len(mutations) if not nxt else m.end() + nxt.start()
    recon_body += xform_storage(mutations[start:end]) + "\n"
    recon_body += textwrap.dedent(
        """
        func (r *Repository) MarkBalancePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
        \treturn groupsql.MarkBalancePaid(ctx, r.pool, groupID, meta, tr)
        }
        """
    )
    recon_header = textwrap.dedent(
        """\
        package storage

        import (
        \t"context"
        \t"errors"
        \t"fmt"
        \t"time"

        \t"github.com/jackc/pgx/v5"
        \t"github.com/jackc/pgx/v5/pgxpool"

        \t"pagasacentre/backend/internal/registration/domain"
        \t"pagasacentre/backend/internal/registration/storage/groupsql"
        )

        type Repository struct {
        \tpool *pgxpool.Pool
        }

        func NewRepository(pool *pgxpool.Pool) *Repository {
        \treturn &Repository{pool: pool}
        }

        """
    )
    write(BILLING / "reconcile/storage/repository.go", recon_header + recon_body)

    # sweep storage
    m = re.search(r"^func \(r \*Repository\) ListOverdueInvoiced\(", bill_repo, re.MULTILINE)
    start = m.start()
    nxt = re.search(r"^func \(r \*Repository\)", bill_repo[m.end() :], re.MULTILINE)
    end = len(bill_repo) if not nxt else m.end() + nxt.start()
    sweep_chunk = bill_repo[start:end].replace("r.db(ctx)", "r.pool")
    sweep_chunk = sweep_chunk.replace("groupSelectCols", "groupsql.GroupSelectCols")
    sweep_chunk = sweep_chunk.replace("scanGroup", "groupsql.ScanGroup")
    sweep_header = textwrap.dedent(
        """\
        package storage

        import (
        \t"context"
        \t"fmt"
        \t"time"

        \t"github.com/jackc/pgx/v5/pgxpool"

        \t"pagasacentre/backend/internal/registration/domain"
        \t"pagasacentre/backend/internal/registration/storage/groupsql"
        )

        type Repository struct {
        \tpool *pgxpool.Pool
        }

        func NewRepository(pool *pgxpool.Pool) *Repository {
        \treturn &Repository{pool: pool}
        }

        """
    )
    write(BILLING / "sweep/storage/repository.go", sweep_header + sweep_chunk)

    write(
        BILLING / "sheetsync/storage/repository.go",
        storage_header()
        + textwrap.dedent(
            """
            func (r *Repository) StampOnly(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
            \treturn groupsql.StampOnly(ctx, r.pool, groupID, meta, tr)
            }
            """
        ),
    )

    combined = svc + "\n" + camper
    routes = {
        "allocation": [
            "Allocate",
            "Unallocate",
            "ConfirmFree",
            "sendSponsorshipConfirmedEmail",
            "VoidAndRelease",
            "camperNames",
        ],
        "invoicing": [
            "SendInvoice",
            "SendInvoicesBulk",
            "coachAlreadyCharged",
            "SendCoachInvoice",
            "SendCoachInvoicesBulk",
            "WaiveCoachFee",
            "UnwaiveCoachFee",
            "MarkBalancePaidManually",
            "sendManualBalancePaidEmails",
            "DeleteRegistration",
            "ResendInvoice",
            "ExtendDueAt",
            "dayPassLine",
            "coachLine",
            "sendAccommodationChangedNotice",
        ],
        "camperedit": [
            "RemoveCamper",
            "ConvertCamperToDayVisitor",
            "UpdateDayPassCamper",
            "UpdateCamperCoach",
            "EditCamper",
            "AddCamper",
            "MakeMainContact",
            "WaiveCamperDeposit",
            "validateEdit",
            "coachEligible",
            "coachEligibleAtAge",
            "campersAfterEdit",
            "findCamper",
            "optStr",
            "resyncSheet",
            "sendBookingUpdated",
        ],
        "reconcile": [
            "HandleCoachInvoicePaid",
            "HandleInvoicePaid",
            "HandleInvoiceFailed",
            "HandleInvoiceUncollectible",
        ],
        "sweep": ["SweepOverdue"],
        "sheetsync": ["ResyncSheet", "ResyncAllSheets"],
    }

    repo_templates = {
        "allocation": read(BILLING / "repo.go"),  # placeholder — overwritten below
    }

    for pkg, funcs in routes.items():
        body = extract_funcs(combined, funcs)
        body = xform_service(body, pkg)
        write(BILLING / pkg / "service.go", SERVICE_IMPORTS[pkg] + "\n" + body)

    print("codegen done — add repo.go + NewService manually if needed")


if __name__ == "__main__":
    main()
