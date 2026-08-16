"""MemberConfig: AgentOrgs Hermes member settings."""
from __future__ import annotations

from pathlib import Path


class WorkerConfig:
    """Config for one Hermes Member (workspace already mirrored by entrypoint)."""

    def __init__(self, worker_name: str, workspace: Path) -> None:
        self.worker_name = worker_name
        self.workspace_dir = workspace

    @property
    def hermes_home(self) -> Path:
        return self.workspace_dir / ".hermes"
