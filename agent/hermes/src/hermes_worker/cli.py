"""CLI entry point: ``hermes-worker`` (AgentOrgs Member runtime)."""
from __future__ import annotations

import asyncio
import logging
import signal
from pathlib import Path

import typer

from hermes_worker.config import WorkerConfig
from hermes_worker.worker import Worker

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)


def main() -> None:
    """Entry point registered in pyproject.toml."""

    def _run(
        name: str = typer.Option(..., "--name", help="Member name"),
        workspace: str = typer.Option(
            "/workspace", "--workspace", help="Local workspace root"
        ),
    ) -> None:
        """Bridge openclaw.json and start the Hermes Matrix gateway."""
        config = WorkerConfig(worker_name=name, workspace=Path(workspace))
        worker = Worker(config)

        async def _async_run() -> None:
            loop = asyncio.get_running_loop()

            def _shutdown() -> None:
                asyncio.create_task(worker.stop())

            try:
                for sig in (signal.SIGINT, signal.SIGTERM):
                    loop.add_signal_handler(sig, _shutdown)
            except NotImplementedError:
                pass

            await worker.run()

        try:
            asyncio.run(_async_run())
        except KeyboardInterrupt:
            pass

    typer.run(_run)
