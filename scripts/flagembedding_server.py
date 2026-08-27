#!/usr/bin/env python3
"""OpenAI-compatible embedding + rerank server backed by FlagEmbedding BGE-M3.

Usage:
  python scripts/flagembedding_server.py --port 8091

FluxSearch settings (local + llamacpp backend):
  embedding_provider: local
  embedding_local_backend: llamacpp
  embedding_api_url: http://127.0.0.1:8091/v1
  embedding_model: bge-m3
  embedding_dim: 1024
  search_hybrid_enabled: true
  search_rerank_enabled: true
  rerank_model: bge-reranker-v2-m3
"""

from __future__ import annotations

import argparse
import json
import math
import os
import sys
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any

_model = None
_sparse_model = None
_reranker = None
_model_name = "bge-m3"
_reranker_name = "bge-reranker-v2-m3"
_encode_lock = threading.Lock()
_sparse_encode_lock = threading.Lock()
_rerank_lock = threading.Lock()
_default_max_length = 512


def load_model(name: str, device: str | None, use_fp16: bool):
    from FlagEmbedding import BGEM3FlagModel

    kwargs: dict[str, Any] = {"use_fp16": use_fp16}
    if device:
        kwargs["device"] = device
    return BGEM3FlagModel(name, **kwargs)


def load_reranker(name: str, device: str | None, use_fp16: bool):
    from FlagEmbedding import FlagReranker

    kwargs: dict[str, Any] = {"use_fp16": use_fp16}
    if device:
        kwargs["device"] = device
    return FlagReranker(name, **kwargs)


def encode_texts(
    texts: list[str],
    batch_size: int,
    max_length: int,
    return_sparse: bool,
) -> list[dict[str, Any]]:
    global _model, _sparse_model
    if _model is None:
        raise RuntimeError("model not loaded")
    with _encode_lock:
        dense_out = _model.encode(
            texts,
            batch_size=batch_size,
            max_length=max_length,
            return_dense=True,
            return_sparse=False,
        )
    dense = dense_out["dense_vecs"]
    sparse_rows = None
    if return_sparse:
        sparse_backend = _sparse_model or _model
        sparse_lock = _sparse_encode_lock if _sparse_model is not None else _encode_lock
        with sparse_lock:
            sparse_out = sparse_backend.encode(
                texts,
                batch_size=batch_size,
                max_length=max_length,
                return_dense=False,
                return_sparse=True,
            )
        sparse_rows = sparse_out.get("lexical_weights")
    rows: list[dict[str, Any]] = []
    for i, vec in enumerate(dense):
        if hasattr(vec, "tolist"):
            raw = vec.tolist()
        else:
            raw = list(vec)
        values = [float(x) for x in raw]
        if any(math.isnan(x) or math.isinf(x) for x in values):
            raise ValueError("embedding produced invalid values (nan/inf)")
        item: dict[str, Any] = {"embedding": values}
        if return_sparse and sparse_rows is not None:
            weights = sparse_rows[i]
            if hasattr(weights, "items"):
                sparse = {str(k): float(v) for k, v in weights.items() if float(v) != 0.0}
            else:
                sparse = {str(k): float(v) for k, v in dict(weights).items() if float(v) != 0.0}
            item["sparse"] = sparse
        rows.append(item)
    return rows


def env_str(key: str, fallback: str) -> str:
    v = os.environ.get(key)
    return v if v is not None and v != "" else fallback


def env_int(key: str, fallback: int) -> int:
    v = os.environ.get(key)
    if v is None or v == "":
        return fallback
    try:
        return int(v)
    except ValueError:
        return fallback


def env_bool(key: str, fallback: bool) -> bool:
    v = os.environ.get(key)
    if v is None or v == "":
        return fallback
    return v.lower() in ("1", "true", "yes", "on")


def rerank_texts(query: str, documents: list[str], top_k: int, max_length: int) -> list[dict[str, Any]]:
    global _reranker
    if _reranker is None:
        raise RuntimeError("reranker not loaded")
    if not documents:
        return []
    pairs = [[query, doc] for doc in documents]
    with _rerank_lock:
        try:
            scores = _reranker.compute_score(pairs, normalize=True, max_length=max_length)
        except TypeError:
            scores = _reranker.compute_score(pairs, normalize=True)
    if isinstance(scores, (int, float)):
        scores = [float(scores)]
    else:
        scores = [float(s) for s in scores]
    ranked = sorted(
        [{"index": i, "score": score} for i, score in enumerate(scores)],
        key=lambda item: item["score"],
        reverse=True,
    )
    if top_k > 0:
        ranked = ranked[:top_k]
    return ranked


class Handler(BaseHTTPRequestHandler):
    server_version = "FlagEmbeddingHTTP/1.0"

    def log_message(self, fmt: str, *args: Any) -> None:
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

    def _read_json(self) -> dict[str, Any]:
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length > 0 else b"{}"
        return json.loads(raw.decode("utf-8"))

    def _json(self, code: int, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, ensure_ascii=False, allow_nan=False).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        if self.path in ("/healthz", "/health"):
            self._json(
                200,
                {
                    "status": "ok",
                    "model": _model_name,
                    "reranker": _reranker_name if _reranker is not None else None,
                },
            )
            return
        if self.path == "/v1/models":
            models = [{"id": _model_name, "object": "model"}]
            if _reranker is not None:
                models.append({"id": _reranker_name, "object": "model"})
            self._json(200, {"data": models})
            return
        self._json(404, {"error": "not found"})

    def do_POST(self) -> None:
        if self.path == "/v1/embeddings":
            self._handle_embeddings()
            return
        if self.path == "/v1/rerank":
            self._handle_rerank()
            return
        self._json(404, {"error": "not found"})

    def _handle_embeddings(self) -> None:
        try:
            body = self._read_json()
            inp = body.get("input", [])
            if isinstance(inp, str):
                texts = [inp]
            else:
                texts = list(inp)
            if not texts:
                self._json(400, {"error": {"message": "input is required"}})
                return
            batch_size = int(body.get("batch_size") or 16)
            raw_max_length = body.get("max_length")
            max_length = int(raw_max_length) if raw_max_length is not None else _default_max_length
            return_sparse = bool(body.get("return_sparse"))
            rows = encode_texts(texts, batch_size=batch_size, max_length=max_length, return_sparse=return_sparse)
            data = [
                {"object": "embedding", "index": i, **row}
                for i, row in enumerate(rows)
            ]
            self._json(200, {"object": "list", "data": data, "model": _model_name})
        except Exception as exc:  # noqa: BLE001
            self._json(500, {"error": {"message": str(exc)}})

    def _handle_rerank(self) -> None:
        try:
            body = self._read_json()
            query = str(body.get("query") or "").strip()
            documents = list(body.get("documents") or [])
            top_k = int(body.get("top_k") or len(documents))
            raw_max_length = body.get("max_length")
            max_length = int(raw_max_length) if raw_max_length is not None else _default_max_length
            if not query:
                self._json(400, {"error": {"message": "query is required"}})
                return
            if not documents:
                self._json(200, {"object": "list", "data": [], "model": _reranker_name})
                return
            data = rerank_texts(query, [str(doc) for doc in documents], top_k, max_length)
            self._json(200, {"object": "list", "data": data, "model": _reranker_name})
        except Exception as exc:  # noqa: BLE001
            self._json(500, {"error": {"message": str(exc)}})


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    if hasattr(sys.stderr, "reconfigure"):
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="FlagEmbedding BGE-M3 OpenAI-compatible server")
    parser.add_argument("--host", default=env_str("FLUXSEARCH_FLAGEMBEDDING_HOST", "127.0.0.1"))
    parser.add_argument("--port", type=int, default=env_int("FLUXSEARCH_FLAGEMBEDDING_PORT", 8091))
    parser.add_argument("--model", default=env_str("FLUXSEARCH_FLAGEMBEDDING_MODEL", "BAAI/bge-m3"), help="HF model id or local path")
    parser.add_argument(
        "--rerank-model",
        default=env_str("FLUXSEARCH_FLAGEMBEDDING_RERANK_MODEL", "BAAI/bge-reranker-v2-m3"),
        help="HF reranker id or local path",
    )
    parser.add_argument("--no-reranker", action="store_true", help="Skip loading reranker model")
    parser.add_argument("--device", default=env_str("FLUXSEARCH_FLAGEMBEDDING_DEVICE", ""), help="Dense embedding + reranker device: cuda / cpu (default: auto)")
    parser.add_argument(
        "--sparse-device",
        default=env_str("FLUXSEARCH_FLAGEMBEDDING_SPARSE_DEVICE", ""),
        help="Sparse (lexical) embedding device; if set, loads a second BGE-M3 on this device for sparse only",
    )
    parser.add_argument("--fp16", action="store_true", help="Use fp16 when CUDA is available")
    parser.add_argument("--max-length", type=int, default=env_int("FLUXSEARCH_FLAGEMBEDDING_MAX_LENGTH", 512))
    args = parser.parse_args()
    if "--fp16" not in sys.argv and env_bool("FLUXSEARCH_FLAGEMBEDDING_FP16", False):
        args.fp16 = True
    if "--no-reranker" not in sys.argv and env_bool("FLUXSEARCH_FLAGEMBEDDING_NO_RERANKER", False):
        args.no_reranker = True

    global _model, _sparse_model, _model_name, _reranker, _reranker_name, _default_max_length
    _model_name = args.model.split("/")[-1] if "/" in args.model else args.model
    _reranker_name = args.rerank_model.split("/")[-1] if "/" in args.rerank_model else args.rerank_model
    _default_max_length = args.max_length
    device = args.device or None
    sparse_device = args.sparse_device or None
    print(f"Loading {_model_name} (dense) on {device or 'auto'} ...", flush=True)
    _model = load_model(args.model, device, use_fp16=args.fp16)
    if sparse_device:
        sparse_fp16 = args.fp16 and sparse_device != "cpu"
        print(f"Loading {_model_name} (sparse) on {sparse_device} ...", flush=True)
        _sparse_model = load_model(args.model, sparse_device, use_fp16=sparse_fp16)
    if not args.no_reranker:
        print(f"Loading reranker {_reranker_name} on {device or 'auto'} ...", flush=True)
        _reranker = load_reranker(args.rerank_model, device, use_fp16=args.fp16)
    print(f"Ready on http://{args.host}:{args.port}/v1/embeddings", flush=True)
    if _reranker is not None:
        print(f"Reranker on http://{args.host}:{args.port}/v1/rerank", flush=True)

    httpd = HTTPServer((args.host, args.port), Handler)
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\nStopped.", flush=True)
    finally:
        httpd.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
