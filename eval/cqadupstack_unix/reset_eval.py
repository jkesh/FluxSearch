#!/usr/bin/env python3
"""Purge eval collection and reset local import state."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from common import (  # noqa: E402
    EVAL_COLLECTION_ID,
    ID_MAP_PATH,
    IMPORT_STATE_PATH,
    resolve_database_url,
    save_json,
)
from purge_collection import main as purge_main  # noqa: E402


def purge_via_sql(database_url: str) -> int:
    try:
        import psycopg2
    except ImportError:
        print("ERROR: psycopg2 not installed. Run: pip install psycopg2-binary")
        return 1

    conn = psycopg2.connect(database_url)
    try:
        with conn:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM documents WHERE collection_id = %s::uuid", (EVAL_COLLECTION_ID,))
                deleted = cur.rowcount
        print(f"Deleted {deleted:,} documents from PostgreSQL (fast SQL purge).")
        print("Note: Milvus vectors are not removed by this path; use ensure-milvus -recreate if needed.")
        return 0
    finally:
        conn.close()


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Reset eval collection and local import artifacts")
    parser.add_argument("--api-url", default="http://localhost:8080")
    parser.add_argument("--workers", type=int, default=4, help="Parallel delete workers for API purge")
    parser.add_argument(
        "--fast",
        action="store_true",
        help="Bulk-delete PostgreSQL rows directly (use after ensure-milvus -recreate)",
    )
    parser.add_argument("--database-url", default="", help="Override PostgreSQL URL for --fast")
    parser.add_argument("--yes", action="store_true")
    args, unknown = parser.parse_known_args()

    if args.fast:
        if not args.yes:
            print("Fast purge deletes all eval documents from PostgreSQL in one SQL statement.")
            if input("Continue? [y/N] ").strip().lower() != "y":
                print("Cancelled.")
                return 1
        database_url = args.database_url or resolve_database_url()
        if not database_url:
            print("ERROR: database URL not found. Set FLUXSEARCH_DATABASE_URL or config/local/infra.env")
            return 1
        code = purge_via_sql(database_url)
    else:
        purge_argv = [
            "purge_collection.py",
            "--api-url",
            args.api_url,
            "--workers",
            str(args.workers),
        ]
        if args.yes:
            purge_argv.append("--yes")

        old_argv = sys.argv
        sys.argv = purge_argv
        try:
            code = purge_main()
        finally:
            sys.argv = old_argv

    if code != 0:
        return code

    save_json(ID_MAP_PATH, {"beir_to_flux": {}, "flux_to_beir": {}})
    save_json(IMPORT_STATE_PATH, {"imported": 0, "failed": 0, "errors": []})
    print(f"Reset {ID_MAP_PATH} and {IMPORT_STATE_PATH}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
