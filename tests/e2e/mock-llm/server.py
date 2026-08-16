#!/usr/bin/env python3
"""E2E OpenAI-compatible mock.

dwmkerr/mock-llm streams only message.content and drops tool_calls. OpenClaw
always sends stream=true, so this server emits real SSE tool_call deltas.
"""

from __future__ import annotations

import json
import re
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HOST = "0.0.0.0"
PORT = 6556
CHUNK_SIZE = 8
CHUNK_INTERVAL_S = 0.01

CONTAINS_RE = re.compile(r"^contains\(to_string\(body\), '([^']*)'\)$")

MODELS = {
    "object": "list",
    "data": [{"id": "gpt-4o-mini", "object": "model", "owned_by": "mock-llm"}],
}


def completion(cid: str, content: str, finish: str = "stop", tool_calls=None) -> dict:
    message = {"role": "assistant", "content": content}
    if tool_calls is not None:
        message["tool_calls"] = tool_calls
    return {
        "id": cid,
        "object": "chat.completion",
        "created": 0,
        "model": "gpt-4o-mini",
        "choices": [{"index": 0, "message": message, "finish_reason": finish}],
        "usage": {"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12},
    }


# Last matching rule wins. "@" always matches.
RULES: list[tuple[str, dict]] = [
    ("@", completion("chatcmpl-agentorgs-e2e", "agentorgs-e2e-ok")),
    (
        "contains(to_string(body), 'E2E_GROUP_TASK')",
        completion(
            "chatcmpl-agentorgs-e2e-group",
            "@worker:matrix-local.agentorgs.io E2E_PEER_TASK please handle",
        ),
    ),
    (
        "contains(to_string(body), 'E2E_PEER_TASK')",
        completion("chatcmpl-agentorgs-e2e-peer", "agentorgs-e2e-peer-ok"),
    ),
    (
        "contains(to_string(body), 'E2E_MEMORY_LEAD')",
        completion(
            "chatcmpl-agentorgs-e2e-oc-mem",
            "",
            finish="tool_calls",
            tool_calls=[
                {
                    "id": "call_e2e_oc_mem",
                    "type": "function",
                    "function": {
                        "name": "write",
                        "arguments": '{"path":"MEMORY.md","content":"E2E_MEMORY_LEAD"}',
                    },
                }
            ],
        ),
    ),
    (
        "contains(to_string(body), 'E2E_MEMORY_WORKER')",
        completion(
            "chatcmpl-agentorgs-e2e-hm-mem",
            "",
            finish="tool_calls",
            tool_calls=[
                {
                    "id": "call_e2e_hm_mem",
                    "type": "function",
                    "function": {
                        "name": "memory",
                        "arguments": '{"action":"add","target":"memory","content":"E2E_MEMORY_WORKER"}',
                    },
                }
            ],
        ),
    ),
    (
        "contains(to_string(body), 'call_e2e_oc_mem')",
        completion("chatcmpl-agentorgs-e2e-oc-mem-done", "agentorgs-e2e-memory-ok"),
    ),
    (
        "contains(to_string(body), 'call_e2e_hm_mem')",
        completion("chatcmpl-agentorgs-e2e-hm-mem-done", "agentorgs-e2e-memory-ok"),
    ),
]


def rule_matches(match: str, body: str) -> bool:
    if match == "@":
        return True
    found = CONTAINS_RE.match(match)
    return bool(found) and found.group(1) in body


def pick_completion(body: str) -> dict:
    chosen = RULES[0][1]
    for match, payload in RULES:
        if rule_matches(match, body):
            chosen = payload
    return json.loads(json.dumps(chosen))


def sse_chunk(cid: str, delta: dict, finish: str | None) -> bytes:
    payload = {
        "id": cid,
        "object": "chat.completion.chunk",
        "created": 0,
        "model": "gpt-4o-mini",
        "choices": [{"index": 0, "delta": delta, "finish_reason": finish}],
    }
    return f"data: {json.dumps(payload, separators=(',', ':'))}\n\n".encode()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt: str, *args) -> None:
        print(f"{self.command} {self.path}")

    def _send(self, status: int, body: bytes, content_type: str) -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _json(self, status: int, obj: dict) -> None:
        self._send(status, json.dumps(obj).encode(), "application/json")

    def do_GET(self) -> None:  # noqa: N802
        path = self.path.split("?", 1)[0].rstrip("/") or "/"
        if path in ("/health", "/ready"):
            self._json(200, {"status": "ready" if path == "/ready" else "healthy"})
            return
        if path == "/v1/models":
            self._json(200, MODELS)
            return
        self._json(404, {"error": "Not Found", "status": 404})

    def do_POST(self) -> None:  # noqa: N802
        path = self.path.split("?", 1)[0].rstrip("/")
        if path != "/v1/chat/completions":
            self._json(404, {"error": "Not Found", "status": 404})
            return
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        try:
            req = json.loads(raw.decode() or "{}")
        except json.JSONDecodeError:
            req = {}
        body_text = raw.decode(errors="replace")
        completion = pick_completion(body_text)
        if req.get("stream") is True:
            self._stream(completion)
            return
        self._json(200, completion)

    def _stream(self, completion: dict) -> None:
        choice = completion["choices"][0]
        message = choice["message"]
        cid = completion["id"]
        tool_calls = message.get("tool_calls") or []
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.end_headers()
        if tool_calls:
            self.wfile.write(sse_chunk(cid, {"role": "assistant"}, None))
            for index, call in enumerate(tool_calls):
                self.wfile.write(
                    sse_chunk(
                        cid,
                        {
                            "tool_calls": [
                                {
                                    "index": index,
                                    "id": call["id"],
                                    "type": call.get("type", "function"),
                                    "function": {
                                        "name": call["function"]["name"],
                                        "arguments": call["function"].get("arguments", ""),
                                    },
                                }
                            ]
                        },
                        None,
                    )
                )
            self.wfile.write(sse_chunk(cid, {}, "tool_calls"))
        else:
            content = message.get("content") or ""
            pieces = [content[i : i + CHUNK_SIZE] for i in range(0, len(content), CHUNK_SIZE)] or [""]
            for i, piece in enumerate(pieces):
                delta: dict = {"content": piece}
                if i == 0:
                    delta["role"] = "assistant"
                finish = "stop" if i == len(pieces) - 1 else None
                self.wfile.write(sse_chunk(cid, delta, finish))
                if i != len(pieces) - 1:
                    time.sleep(CHUNK_INTERVAL_S)
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()


def main() -> None:
    httpd = ThreadingHTTPServer((HOST, PORT), Handler)
    print(f"agentorgs mock-llm streaming tool_calls on {HOST}:{PORT}")
    httpd.serve_forever()


if __name__ == "__main__":
    main()
