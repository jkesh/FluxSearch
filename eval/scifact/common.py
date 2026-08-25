"""Shared constants and retrieval metrics for BEIR SciFact eval."""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Iterable

EVAL_COLLECTION_ID = "00000000-0000-0000-0000-0000000000e2"
EVAL_MILVUS_COLLECTION = "fluxsearch_eval_scifact"
DATASET_NAME = "scifact"

ROOT = Path(__file__).resolve().parents[2]
ENV_FILE = ROOT / "config" / "local" / "infra.env"
DATA_DIR = ROOT / "eval" / "data" / DATASET_NAME
CORPUS_PATH = DATA_DIR / "corpus.jsonl"
QUERIES_PATH = DATA_DIR / "queries.jsonl"
QRELS_PATH = DATA_DIR / "qrels.json"
ID_MAP_PATH = DATA_DIR / "id_map.json"
IMPORT_STATE_PATH = DATA_DIR / "import_state.json"
IMPORT_LOCK_PATH = DATA_DIR / "import.lock"
REPORT_DIR = ROOT / "eval" / "reports"


def load_env_file(path: Path | None = None) -> dict[str, str]:
    values: dict[str, str] = {}
    env_path = path or ENV_FILE
    if not env_path.exists():
        return values
    for line in env_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip()
    return values


def resolve_database_url(env: dict[str, str] | None = None) -> str:
    env = env or load_env_file()
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


def ensure_data_dir() -> Path:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    REPORT_DIR.mkdir(parents=True, exist_ok=True)
    return DATA_DIR


def load_json(path: Path, default):
    if not path.exists():
        return default
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def save_json(path: Path, payload) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=2)


def merge_id_maps(
    local: dict[str, dict[str, str]],
    on_disk: dict[str, dict[str, str]] | None = None,
) -> dict[str, dict[str, str]]:
    disk = on_disk if on_disk is not None else load_json(ID_MAP_PATH, {"beir_to_flux": {}, "flux_to_beir": {}})
    merged_b2f = dict(disk.get("beir_to_flux", {}))
    merged_b2f.update(local.get("beir_to_flux", {}))
    merged_f2b = {flux_id: beir_id for beir_id, flux_id in merged_b2f.items()}
    return {"beir_to_flux": merged_b2f, "flux_to_beir": merged_f2b}


def import_lock_active() -> bool:
    if not IMPORT_LOCK_PATH.exists():
        return False
    try:
        pid = int(IMPORT_LOCK_PATH.read_text(encoding="utf-8").strip())
    except ValueError:
        return True
    if pid <= 0:
        return True
    try:
        import os

        os.kill(pid, 0)
    except OSError:
        IMPORT_LOCK_PATH.unlink(missing_ok=True)
        return False
    return True


def iter_jsonl(path: Path) -> Iterable[dict]:
    with path.open("r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                yield json.loads(line)


def hit_at_k(ranked_ids: list[str], relevant: set[str], k: int) -> float:
    return float(any(doc_id in relevant for doc_id in ranked_ids[:k]))


def mrr_at_k(ranked_ids: list[str], relevant: set[str], k: int) -> float:
    for i, doc_id in enumerate(ranked_ids[:k], start=1):
        if doc_id in relevant:
            return 1.0 / i
    return 0.0


def recall_at_k(ranked_ids: list[str], relevant: set[str], k: int) -> float:
    if not relevant:
        return 0.0
    found = sum(1 for doc_id in ranked_ids[:k] if doc_id in relevant)
    return found / len(relevant)
