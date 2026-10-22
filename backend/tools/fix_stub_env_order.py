#!/usr/bin/env python3
"""Move stub (+ optional var captured / mailer / voided) before env in billing integration tests."""
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def stub_block_start(chunk: str, stub_i: int) -> int:
    start = stub_i
    while True:
        prev = chunk.rfind("\n", 0, start - 1) + 1
        if prev == start:
            break
        line = chunk[prev:start].rstrip("\n")
        if re.match(r"\tvar (captured|capturedType|voided)", line) or line.startswith("\tmailer :="):
            start = prev
            continue
        break
    return start


def stub_block_end(chunk: str, stub_i: int) -> int:
    j = chunk.find("{", stub_i)
    depth = 0
    k = j
    while k < len(chunk):
        if chunk[k] == "{":
            depth += 1
        elif chunk[k] == "}":
            depth -= 1
            if depth == 0:
                return k + 1
        k += 1
    return stub_i


def fix_chunk(chunk: str) -> str:
    env_i = chunk.find("env := billingtest.NewEnv(t, pool, stub,")
    stub_i = chunk.find("stub := &billingtest.StubStripe")
    if env_i == -1 or stub_i == -1 or stub_i < env_i:
        return chunk
    sb_start = stub_block_start(chunk, stub_i)
    sb_end = stub_block_end(chunk, stub_i)
    stub_block = chunk[sb_start:sb_end]
    if not stub_block.endswith("\n"):
        stub_block += "\n"
    env_end = chunk.find("\n", env_i) + 1
    if chunk[env_end : env_end + 16].startswith("\trepo := env.Repo"):
        env_end = chunk.find("\n", env_end) + 1
    env_block = chunk[env_i:env_end]
    middle = chunk[env_end:sb_start]
    return chunk[:env_i] + stub_block + env_block + middle + chunk[sb_end:]


def fix_file(rel: str) -> None:
    p = ROOT / rel
    parts = re.split(r"(?m)(?=^func )", p.read_text())
    p.write_text("".join(fix_chunk(c) for c in parts))


if __name__ == "__main__":
    for rel in (
        "backend/internal/billing/invoicing/service_test.go",
        "backend/internal/billing/camperedit/service_test.go",
    ):
        fix_file(rel)
    print("stub/env order fixed")
