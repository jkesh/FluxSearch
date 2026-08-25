#!/usr/bin/env python3
"""Create the SciFact eval PostgreSQL collection row (required before corpus import)."""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MIGRATION = ROOT / "migrations" / "007_eval_scifact.sql"
ENV_FILE = ROOT / "config" / "local" / "infra.env"

from common import EVAL_COLLECTION_ID, EVAL_MILVUS_COLLECTION  # noqa: E402


def load_env_file(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    if not path.exists():
        return values
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip()
    return values


def resolve_database_url(env: dict[str, str]) -> str:
    if url := os.environ.get("FLUXSEARCH_DATABASE_URL"):
        return url
    if url := env.get("FLUXSEARCH_DATABASE_URL_LOCAL"):
        return url
    if url := env.get("FLUXSEARCH_DATABASE_URL"):
        return url

    host = env.get("FLUXSEARCH_POSTGRES_HOST_LOCAL") or env.get("FLUXSEARCH_POSTGRES_HOST", "127.0.0.1")
    port = env.get("FLUXSEARCH_POSTGRES_PORT_LOCAL") or env.get("FLUXSEARCH_POSTGRES_PORT", "5432")
    user = env.get("FLUXSEARCH_POSTGRES_USER", "fluxsearch")
    password = env.get("FLUXSEARCH_POSTGRES_PASSWORD", "")
    db = env.get("FLUXSEARCH_POSTGRES_DB", "fluxsearch")
    return f"postgresql://{user}:{password}@{host}:{port}/{db}"


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Apply SciFact eval collection migration to PostgreSQL")
    parser.add_argument("--database-url", default="", help="Override PostgreSQL URL")
    parser.add_argument("--dry-run", action="store_true", help="Print SQL only")
    args = parser.parse_args()

    sql = MIGRATION.read_text(encoding="utf-8")
    if args.dry_run:
        print(sql)
        return 0

    database_url = args.database_url or resolve_database_url(load_env_file(ENV_FILE))
    if not database_url:
        print("ERROR: database URL not found. Set FLUXSEARCH_DATABASE_URL or config/local/infra.env")
        return 1

    try:
        import psycopg2
    except ImportError:
        print("ERROR: psycopg2 not installed. Run: pip install psycopg2-binary")
        print("\nOr apply manually:")
        print(f"  psql \"$FLUXSEARCH_DATABASE_URL\" -f {MIGRATION}")
        return 1

    print(f"Applying migration to eval collection {EVAL_COLLECTION_ID} ...")
    conn = psycopg2.connect(database_url)
    try:
        with conn:
            with conn.cursor() as cur:
                cur.execute(sql)
        print("PostgreSQL collection ready.")
    finally:
        conn.close()

    print("\nNext: create Milvus collection (reads embedding_dim from app.settings.json)")
    print(f"  $env:FLUXSEARCH_MILVUS_COLLECTION = \"{EVAL_MILVUS_COLLECTION}\"")
    print("  go run ./cmd/ensure-milvus -recreate")
    print("\nThen restart the API (go run ./cmd/api) before importing.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
