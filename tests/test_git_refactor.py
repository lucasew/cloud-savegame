import shutil
from unittest.mock import MagicMock, patch

from cloud_savegame.git import GitManager


def test_git_manager_init():
    # If git is available
    if shutil.which("git"):
        mgr = GitManager(enabled=True)
        assert mgr.enabled
        assert mgr.binary is not None

        mgr_disabled = GitManager(enabled=False)
        assert not mgr_disabled.enabled
    else:
        # If git is not available, enabled=True should raise
        try:
            GitManager(enabled=True)
            assert False, "Should raise AssertionError"
        except AssertionError:
            pass


@patch("cloud_savegame.git.subprocess")
def test_git_manager_run(mock_subprocess):
    mgr = GitManager(enabled=True)
    mgr.binary = "/usr/bin/git"  # Force a path for testing

    mgr.run("status")
    mock_subprocess.call.assert_called_with(["/usr/bin/git", "status"])


@patch("cloud_savegame.git.subprocess")
def test_git_manager_is_dirty(mock_subprocess):
    mgr = GitManager(enabled=True)
    mgr.binary = "/usr/bin/git"

    # Mock return value for clean repo
    mock_subprocess.run.return_value = MagicMock(stdout="")
    assert mgr.is_dirty() is False

    # Mock return value for dirty repo
    mock_subprocess.run.return_value = MagicMock(stdout="M file.txt")
    assert mgr.is_dirty() is True
