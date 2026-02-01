import logging
import subprocess
from shutil import which

logger = logging.getLogger(__name__)


class GitManager:
    def __init__(self, enabled: bool):
        self.binary = which("git")
        if enabled and self.binary is None:
            raise AssertionError("git required but not available")
        self.enabled = enabled

    def run(self, *params, always_show=False) -> None:
        """
        Execute a git command with the provided parameters.

        Args:
            *params: Command line arguments to pass to git.
            always_show: Unused parameter (legacy).
        """
        if not self.enabled or self.binary is None:
            return

        logger.info("git: %s", " ".join(f"'{p}'" for p in params))
        subprocess.call([self.binary, *params])

    def is_dirty(self) -> bool:
        """
        Check if the current git repository has uncommitted changes.

        Returns:
            bool: True if `git status -s` returns any output (indicating dirtiness),
            False otherwise.
        """
        if not self.enabled or self.binary is None:
            return False

        status_result = subprocess.run(
            [self.binary, "status", "-s"], capture_output=True, text=True
        )
        return bool(status_result.stdout)
