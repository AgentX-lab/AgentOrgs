"""Hermes Member entry: bridge workspace openclaw.json, then run gateway.

MinIO sync is owned by the container entrypoint (mc mirror), not this module.
"""
from __future__ import annotations

import asyncio
import json
import logging
import os
import shutil
from pathlib import Path
from typing import Any, Dict, Optional

from rich.console import Console
from rich.panel import Panel

from hermes_worker.bridge import bridge_openclaw_to_hermes
from hermes_worker.config import WorkerConfig

console = Console()
logger = logging.getLogger(__name__)


class Worker:
    """Owns bridge + hermes-agent gateway for one Member."""

    def __init__(self, config: WorkerConfig) -> None:
        self.config = config
        self.worker_name = config.worker_name
        self._hermes_home: Path = config.hermes_home
        self._gateway_task: Optional[asyncio.Task] = None
        self._stopping = False

    async def run(self) -> None:
        if not await self.start():
            return
        try:
            await self._run_hermes_gateway()
        except asyncio.CancelledError:
            pass
        finally:
            await self.stop()

    async def stop(self) -> None:
        if self._stopping:
            return
        self._stopping = True
        console.print("[yellow]Stopping hermes member...[/yellow]")
        if self._gateway_task and not self._gateway_task.done():
            self._gateway_task.cancel()
            try:
                await self._gateway_task
            except (asyncio.CancelledError, Exception):
                pass
        console.print("[green]Hermes member stopped.[/green]")

    async def start(self) -> bool:
        workspace = self.config.workspace_dir
        console.print(
            Panel.fit(
                f"[bold green]Hermes Member[/bold green]\n"
                f"Member: [cyan]{self.worker_name}[/cyan]\n"
                f"Workspace: [cyan]{workspace}[/cyan]\n"
                f"HERMES_HOME: [cyan]{self._hermes_home}[/cyan]",
                title="Starting",
            )
        )

        cfg_path = workspace / "openclaw.json"
        try:
            openclaw_cfg: Dict[str, Any] = json.loads(
                cfg_path.read_text(encoding="utf-8")
            )
        except Exception as exc:
            console.print(f"[red]Failed to read {cfg_path}: {exc}[/red]")
            return False

        self._hermes_home.mkdir(parents=True, exist_ok=True)
        os.environ["HERMES_HOME"] = str(self._hermes_home)

        console.print("[yellow]Bridging openclaw.json → hermes config...[/yellow]")
        try:
            soul = self._read_text_file(workspace / "SOUL.md") or ""
            agents = self._read_text_file(workspace / "AGENTS.md") or ""
            bridge_openclaw_to_hermes(
                openclaw_cfg,
                self._hermes_home,
                soul=soul or None,
                agents_md=agents or None,
            )
        except Exception as exc:
            console.print(f"[red]Bridge failed: {exc}[/red]")
            return False

        self._load_env_file(self._hermes_home / ".env")
        self._sync_skills()
        self._copy_mcporter_config()
        console.print("[bold green]Hermes member initialized.[/bold green]")
        return True

    async def _run_hermes_gateway(self) -> None:
        from gateway.config import load_gateway_config
        from gateway.run import start_gateway

        gw_config = load_gateway_config()
        console.print(
            f"[bold green]Starting hermes gateway "
            f"(home={self._hermes_home})[/bold green]"
        )
        self._gateway_task = asyncio.create_task(
            start_gateway(gw_config, replace=False, verbosity=0)
        )
        try:
            await self._gateway_task
        except asyncio.CancelledError:
            raise
        except Exception as exc:
            console.print(f"[red]hermes gateway crashed: {exc}[/red]")
            raise

    def _sync_skills(self) -> None:
        skills_dir = self._hermes_home / "skills"
        skills_dir.mkdir(parents=True, exist_ok=True)
        src_root = self.config.workspace_dir / "skills"
        if not src_root.is_dir():
            return

        installed: list[str] = []
        for src_dir in src_root.iterdir():
            if not src_dir.is_dir():
                continue
            name = src_dir.name
            dst_dir = skills_dir / name
            dst_dir.mkdir(parents=True, exist_ok=True)
            for src_file in src_dir.rglob("*"):
                if not src_file.is_file():
                    continue
                rel = src_file.relative_to(src_dir)
                dst_file = dst_dir / rel
                dst_file.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(src_file, dst_file)
                if dst_file.suffix == ".sh":
                    dst_file.chmod(dst_file.stat().st_mode | 0o111)
            installed.append(name)

        if installed:
            console.print(f"[green]Skills installed: {', '.join(installed)}[/green]")

        keep = set(installed)
        for child in list(skills_dir.iterdir()):
            if child.is_dir() and child.name not in keep:
                try:
                    shutil.rmtree(child)
                except OSError as exc:
                    logger.debug("Could not remove %s: %s", child, exc)

    def _copy_mcporter_config(self) -> None:
        src = self.config.workspace_dir / "config" / "mcporter.json"
        if not src.exists():
            return
        dst = self._hermes_home / "config" / "mcporter.json"
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(src, dst)

    @staticmethod
    def _read_text_file(path: Path) -> Optional[str]:
        try:
            return path.read_text() if path.exists() else None
        except OSError:
            return None

    @staticmethod
    def _load_env_file(env_path: Path) -> None:
        if not env_path.exists():
            return
        try:
            for raw in env_path.read_text(errors="replace").splitlines():
                line = raw.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, _, val = line.partition("=")
                key = key.strip()
                val = val.strip()
                if (val.startswith('"') and val.endswith('"')) or (
                    val.startswith("'") and val.endswith("'")
                ):
                    val = val[1:-1]
                    val = val.replace('\\"', '"').replace("\\\\", "\\")
                os.environ[key] = val
        except OSError as exc:
            logger.warning("Could not source %s: %s", env_path, exc)
