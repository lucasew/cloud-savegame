#!/usr/bin/env python3

import logging
import os
import re
import socket
import subprocess
from argparse import ArgumentDefaultsHelpFormatter, ArgumentParser
from collections import defaultdict
from configparser import ConfigParser
from pathlib import Path
from pprint import pformat
from shutil import SameFileError, copyfile, which
from time import time
from typing import Iterator, List, Optional, Set, Tuple

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Constants
DEFAULT_CONFIG_FILE = Path(__file__).parents[0] / "demo.cfg"
HOMEFINDER_FIND_FOLDERS = [".config", "AppData"]
HOMEFINDER_IGNORE_FOLDERS = ["dosdevices", "nixpkgs", ".git", ".cache"]
HOMEFINDER_DOCUMENTS_FOLDER = ["Documentos", "Documents"]
GIT_BIN = which("git")
NEWS_LIST = []


# Function to run git commands
def git(*params, always_show=False) -> None:
    """
    Run git with the parameters if it's enabled
    Noop if git is disabled
    """
    if GIT_BIN is None:
        return
    logger.info("git: %s", " ".join(f"'{p}'" for p in params))
    subprocess.call([GIT_BIN, *params])


# Function to check if the Git repo has uncommitted files
def git_is_repo_dirty() -> bool:
    """
    Check if the Git repo has uncommitted files
    """
    status_result = subprocess.run(["git", "status", "-s"], capture_output=True, text=True)
    return bool(status_result.stdout)


# Function to move a file or folder to a backup directory to prevent accidental deletion
def backup_item(item: Path, output_dir: Path) -> None:
    """
    Move a file or folder to a backup directory to prevent accidental deletion.
    """
    backup_dir = output_dir / "__backup__"
    backup_dir.mkdir(exist_ok=True, parents=True)

    # Add timestamp to avoid name collisions
    backup_target = backup_dir / f"{item.name}.{int(time())}"

    item = Path(item)
    item.rename(backup_target)
    logger.info(f"Moved '{item}' to backup at '{backup_target}'")
    warning_news(
        f"Moved potentially conflicting item '{item}' to the backup directory at '{backup_target}'."
    )


# Function to add a message to the news list and log it as a warning
def warning_news(message: str) -> None:
    """
    Add a message to the news list and log it as a warning
    """
    NEWS_LIST.append(message)
    logger.warning(message)


# Function to get hostname of this machine to report it in the commit
def get_hostname() -> str:
    """
    Get hostname of this machine to report it in the commit
    """
    return socket.gethostname()


# Config file helpers
def get_str(config: ConfigParser, section: str, key: str) -> Optional[str]:
    if section not in config or key not in config[section]:
        return None
    return config[section][key]


def get_list(config: ConfigParser, section: str, key: str) -> Optional[List[str]]:
    divider = get_str(config, "general", "divider") or ","
    raw = get_str(config, section, key) or ""
    raw = raw.strip()
    if not raw:
        return None
    return list(raw.split(divider))


def get_paths(config: ConfigParser, section: str, key: str) -> Set[Path]:
    ret = []
    for p in get_list(config, section, key) or []:
        ret.append(Path(os.path.expanduser(p)).resolve())
    return set(ret)


def get_bool(config: ConfigParser, section: str, key: str) -> bool:
    return get_str(config, section, key) is not None


# Main function to handle the backup process
def main() -> None:
    global GIT_BIN
    config = ConfigParser()
    config["general"] = {}
    config["general"]["divider"] = ","

    parser = ArgumentParser(
        formatter_class=ArgumentDefaultsHelpFormatter,
        prog="cloud-savegame",
        description="Backs up games saved data",
    )

    parser.add_argument(
        "-c",
        "--config",
        type=Path,
        help="Configuration file to be used by the application",
        default=DEFAULT_CONFIG_FILE,
    )
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        help="Which folder to copy backed up files",
        required=True,
    )
    parser.add_argument(
        "-v",
        "--verbose",
        help="Give more detail about what is happening",
        action="store_true",
    )
    parser.add_argument("-g", "--git", help="Use git for snapshot", action="store_true")
    parser.add_argument(
        "-b",
        "--backlink",
        help="Create symlinks at the origin pointing to the repo",
        action="store_true",
    )
    parser.add_argument(
        "--max-depth",
        dest="max_depth",
        help="Max depth for filesystem searches",
        type=int,
        default=10,
    )

    args = parser.parse_args()

    if args.git and GIT_BIN is None:
        raise AssertionError("git required but not available")

    if not args.git:
        GIT_BIN = None

    if not args.config.is_file():
        raise AssertionError("Configuration file is not actually a file")

    if not (args.output.is_dir() or not args.output.exists()):
        raise AssertionError("Output folder is not actually a folder")

    if not args.output.exists():
        args.output.mkdir(exist_ok=True, parents=True)

    args.output = args.output.resolve()

    if args.verbose:
        logging.root.setLevel(logging.DEBUG)

    logger.debug("loading configuration file")
    config.read(args.config)

    hostname = get_hostname()
    ignored_paths = get_paths(config, "search", "ignore")
    logger.debug("parsed config file:")
    logger.debug({section: dict(config[section]) for section in config.sections()})

    start_time = time()

    os.chdir(str(args.output))

    # Initialize git repository if needed
    if args.git:
        if not (args.output / ".git").exists():
            git("init", "--initial-branch", "master")
        is_repo_initially_dirty = git_is_repo_dirty()
        if is_repo_initially_dirty:
            git("add", "-A")
            git("stash", "push")
            git("stash", "pop")
            git("add", "-A")
            git("commit", "-m", f"dirty repo state from hostname {hostname}")

    # Function to check if a path is in the list of ignored paths
    def is_path_ignored(path) -> bool:
        """
        Is the path in the list of ignored paths?
        """
        path_str = str(path)
        return any(path_str.startswith(str(ignored)) for ignored in ignored_paths)

    # Function to copy either a file or a folder from source to destination
    def copy_item(input_item: Path, destination: Path, depth: int = 0) -> None:
        """
        Copy either a file or a folder from source to destination
        """
        original_input_item = input_item
        input_item = Path(input_item.resolve())
        destination = Path(destination.resolve())

        if args.verbose:
            logger.debug(f"Evaluating copy: {input_item} -> {destination}")

        if not input_item.exists():
            return

        if str(input_item).startswith(str(args.output)):
            logger.warning(
                (" " * depth) + f"copy_item: Not copying '{input_item}': Origin is inside output"
            )
            return

        if original_input_item.is_symlink():
            logger.warning(
                (" " * depth) + f"copy_item: Not copying '{input_item}' because it's a symlink"
            )
            return

        if input_item.is_file():
            destination.parent.mkdir(exist_ok=True, parents=True)
            if destination.exists():
                if input_item.stat().st_mtime < destination.stat().st_mtime:
                    if args.verbose:
                        logger.debug(
                            (" " * depth) + f"copy_item: Not copying '{input_item}': Didn't change"
                        )
                    return
            logger.info((" " * depth) + f"copy_item: Copying '{input_item}' to '{destination}'")
            try:
                copyfile(input_item, destination)
            except SameFileError:
                pass

        elif input_item.is_dir():
            destination.mkdir(exist_ok=True, parents=True)
            for item in input_item.iterdir():
                copy_item(input_item / item.name, destination / item.name, depth=depth + 1)

    # Function to ingest a path for an app and rulename
    def ingest_path(
        app: str,
        rule_name: str,
        path: str,
        top_level: bool = False,
        base_path: Optional[Path] = None,
    ) -> None:
        """
        Ingest a path for an app and rulename

        top_level is a strategy to keep track of items for the backlink feature
        """
        try:
            # Security: If a base_path is provided, resolve the input path and ensure
            # it is a child of the base_path. This prevents path traversal.
            if base_path:
                try:
                    resolved_path = Path(path).resolve()
                    if not str(resolved_path).startswith(str(base_path.resolve())):
                        warning_news(
                            f"Security: Path '{path}' for app '{app}' resolves outside of its "
                            f"base '{base_path}'. Skipping."
                        )
                        return
                except FileNotFoundError:
                    # Path might be a glob that doesn't exist yet. Check will happen on recursion.
                    pass
            # Security: Disallow absolute paths that don't come from a variable,
            # as they are untrusted.
            elif "*" not in path and Path(path).is_absolute():
                warning_news(
                    f"Security: Absolute path '{path}' for app '{app}' "
                    "is not allowed in rules. Skipping."
                )
                return

            if is_path_ignored(path):
                return

            path = str(path)
            ppath = Path(path)

            safe_base_dir = (args.output / app).resolve()
            output_dir = (args.output / app / rule_name).resolve()

            # Security: Ensure the output directory is within the intended app directory
            # to prevent path traversal attacks.
            safe_base_dir_str = str(safe_base_dir)
            if not (str(output_dir) == safe_base_dir_str or str(output_dir).startswith(safe_base_dir_str + os.path.sep)):
                warning_news(
                    f"Security: Path traversal attempt detected in rule '{rule_name}' for app '{app}'. Skipping."
                )
                continue

            if "*" in path:
                top_level = False
                filename = ppath.name
                parent = ppath.parent

                if "*" in str(parent):
                    raise AssertionError(
                        f"globs in any path segment but the last are unsupported. "
                        f"This is a rule bug. app={app} rule_name={rule_name} path='{path}'"
                    )

                names = set([x.name for x in [*parent.glob(filename), *output_dir.glob(filename)]])

                for name in names:
                    item = parent / name
                    new_rule_name = str(Path(rule_name) / item.name)
                    ingest_path(
                        app,
                        new_rule_name,
                        str(parent / name),
                        top_level=True,
                        base_path=base_path or parent,
                    )

            elif ppath.exists():
                logger.info(f"ingest '{path}' '{str(output_dir)}'")
                copy_item(ppath, output_dir)

                if args.git and git_is_repo_dirty():
                    commit = f"hostname={hostname} app={app} rule={rule_name} path={path}"
                    git("add", "-A")
                    git("commit", "-m", commit)

            # backlink logic
            if args.backlink and top_level:
                logger.debug(f"TOPLEVEL: {app} {rule_name} {path} {Path(path).resolve()}")
                ppath.parent.mkdir(parents=True, exist_ok=True)

                if ppath.is_symlink():
                    ppath.unlink()  # recreate
                elif ppath.exists():
                    backup_item(ppath, args.output)

                logger.info(f"ln {ppath} -> {output_dir}")
                ppath.symlink_to(output_dir)

            if ppath.is_symlink() and not ppath.exists():
                warning_news(
                    f"This may be a rule or a program bug: '{ppath}' points to a non existent location."  # noqa:E501
                )

        except Exception as e:
            warning_news(f"while ingesting app={app} rule={rule_name} path={path}: {type(e)} {e}")

    RULES_DIR = [Path(__file__).parents[0] / "rules", args.output / "__rules__"]
    RULES_DIR[1].mkdir(exist_ok=True, parents=True)

    apps = set()
    required_vars = defaultdict(set)
    var_users = defaultdict(set)
    all_vars = set()
    rulefiles = {}

    # Function to parse rules from one app
    def parse_rules(app: str) -> Iterator[Tuple[str, str]]:
        """
        Parse rules from one app
        """
        rulefile = rulefiles[app]
        logger.debug(f"loading rule '{rulefile}'")
        for line in Path(rulefile).read_text().split("\n"):
            rule = line.strip()
            if rule:
                parts = rule.split(" ", 1)  # Split only on first space
                if len(parts) == 2:
                    rule_name, rule_path = parts
                    if not get_bool(config, app, f"ignore_{rule_name}"):
                        yield rule_name.strip(), rule_path.strip()

    # Load rules
    rules_amount = 0

    for ruledir in RULES_DIR:
        if not ruledir.is_dir():
            continue

        for rulefile in ruledir.glob("*.txt"):
            appname = rulefile.stem
            apps.add(appname)
            rulefiles[appname] = rulefile

            for rule_name, rule_path in parse_rules(appname):
                match = re.search(r"\$([a-z]*)", rule_path)
                variables = list(match.groups()) if match else []

                if not variables:
                    ingest_path(appname, rule_name, rule_path)
                    continue

                for var in variables:
                    required_vars[appname].add(var)
                    all_vars.add(var)
                    var_users[var].add(appname)

                rules_amount += 1

    logger.debug(f"loaded {rules_amount} rules for {len(apps)} apps")
    logger.debug(f"all apps with rules loaded: {pformat(apps)}")
    logger.debug(f"all variables mentioned in rules: {all_vars}")

    # Process games that use installdir variable
    for game in var_users["installdir"]:
        game_install_dirs = get_paths(config, game, "installdir")
        if not game_install_dirs:
            if get_str(config, game, "not_installed") is None:
                warning_news(
                    f"installdir missing for game {game}, please add it in the game configuration section "  # noqa: E501
                    f"or set anything to not_installed to disable this warning"
                )
            continue

        for game_install_dir in game_install_dirs:
            if not game_install_dir.exists():
                warning_news(f"Game install dir for {game} doesn't exist: {game_install_dir}")
                continue

            if is_path_ignored(game_install_dir):
                continue

            for rule_name, rule_path in parse_rules(game):
                resolved_rule_path = rule_path.replace(
                    "$installdir", str(game_install_dir.resolve())
                )

                if rule_path == resolved_rule_path:
                    continue

                ingest_path(
                    game, rule_name, resolved_rule_path, top_level=True, base_path=game_install_dir
                )

    # Function to search for home directories
    def search_for_homes(start_dir: Path, max_depth: int = args.max_depth) -> Iterator[Path]:
        """
        Return an iterator of home dirs found starting from one start_dir
        """
        if (
            max_depth <= 0
            or start_dir.is_symlink()
            or not start_dir.is_dir()
            or is_path_ignored(start_dir)
            or start_dir.name in HOMEFINDER_IGNORE_FOLDERS
        ):
            return

        try:
            for pattern in HOMEFINDER_FIND_FOLDERS:
                if (start_dir / pattern).exists():
                    yield start_dir
                    break

            for item in start_dir.iterdir():
                yield from search_for_homes(item, max_depth=max_depth - 1)

        except PermissionError:
            pass

    # Function to get all home directories
    def get_homes() -> Iterator[Path]:
        """
        Get all homes using data from the config file and search_for_homes
        """
        extra_homes = get_paths(config, "search", "extra_homes")
        if extra_homes:
            for home in extra_homes:
                if is_path_ignored(home):
                    continue

                if not home.exists():
                    warning_news(f"extra home '{str(home)}' does not exist")
                else:
                    yield home

        for search_path in get_paths(config, "search", "paths"):
            yield from search_for_homes(search_path)

    ALL_HOMES = []
    try:
        for homedir in get_homes():
            if is_path_ignored(homedir):
                continue

            ALL_HOMES.append(homedir)
            logger.debug(f"Looking for stuff in {str(homedir)}")

            # Process home variable
            for game in var_users.get("home") or []:
                for rule_name, rule_path in parse_rules(game):
                    resolved_rule_path = rule_path.replace("$home", str(homedir))
                    if rule_path != resolved_rule_path:
                        ingest_path(
                            game, rule_name, resolved_rule_path, top_level=True, base_path=homedir
                        )

            # Process appdata variable
            for game in var_users["appdata"]:
                appdata = homedir / "AppData"
                for rule_name, rule_path in parse_rules(game):
                    resolved_rule_path = rule_path.replace("$appdata", str(appdata))
                    if rule_path != resolved_rule_path:
                        ingest_path(
                            game, rule_name, resolved_rule_path, top_level=True, base_path=appdata
                        )

            # Find program files and process related variables
            has_program_files = False
            for program_files_candidate in homedir.parent.parent.iterdir():
                try:
                    if not (program_files_candidate / "Common Files").exists():
                        continue

                    has_program_files = True
                    program_files = program_files_candidate

                    # Process program_files variable
                    for game in var_users["program_files"]:
                        for rule_name, rule_path in parse_rules(game):
                            resolved_rule_path = rule_path.replace(
                                "$program_files", str(program_files)
                            )
                            if rule_path != resolved_rule_path:
                                ingest_path(
                                    game,
                                    rule_name,
                                    resolved_rule_path,
                                    top_level=True,
                                    base_path=program_files,
                                )

                    # Process Ubisoft specific files
                    ubisoft_users = set()
                    ubisoft_specific_dir = args.output / "ubisoft"
                    ubisoft_specific_dir.mkdir(parents=True, exist_ok=True)
                    ubisoft_users_file = ubisoft_specific_dir / "users.txt"

                    if ubisoft_users_file.exists():
                        content = ubisoft_users_file.read_text().strip()
                        if content:
                            ubisoft_users.update(content.split("\n"))

                    ubisoft_savegame_dir = (
                        program_files / "Ubisoft" / "Ubisoft Game Launcher" / "savegames"
                    )

                    if ubisoft_savegame_dir.exists():
                        for ubisoft_user in ubisoft_savegame_dir.iterdir():
                            if ubisoft_user.is_dir():
                                logger.debug(f"UBISOFT/iterdir: {ubisoft_user}")
                                ubisoft_users.add(ubisoft_user.name)

                    ubisoft_users_file.write_text("\n".join(list(ubisoft_users)))

                    # Process ubisoft variable
                    for ubisoft_user in ubisoft_users:
                        ubisoft_var = ubisoft_savegame_dir / ubisoft_user
                        for game in var_users["ubisoft"]:
                            for rule_name, rule_path in parse_rules(game):
                                resolved_rule_path = rule_path.replace("$ubisoft", str(ubisoft_var))
                                if rule_path != resolved_rule_path:
                                    logger.debug(f"UBISOFT {resolved_rule_path} {ubisoft_users}")
                                    ingest_path(
                                        game,
                                        rule_name,
                                        resolved_rule_path,
                                        top_level=True,
                                        base_path=ubisoft_var,
                                    )

                except PermissionError:
                    has_program_files = True
                    continue

            if not has_program_files:
                warning_news(
                    f"home '{homedir}' is neither a Linux home nor has a Windows-like Program Files."  # noqa: E501
                    f"This is a bug and a proof that this implementation is incomplete. Please report."  # noqa: E501
                    f"Context: https://twitter.com/lucas59356/status/1700965748611449086."
                )

            # Process documents variable
            for documents_candidate in HOMEFINDER_DOCUMENTS_FOLDER:
                documents = homedir / documents_candidate
                if not documents.exists():
                    continue

                for game in var_users["documents"]:
                    for rule_name, rule_path in parse_rules(game):
                        resolved_rule_path = rule_path.replace("$documents", str(documents))
                        if rule_path != resolved_rule_path:
                            ingest_path(
                                game,
                                rule_name,
                                resolved_rule_path,
                                top_level=True,
                                base_path=documents,
                            )

    finally:
        logger.info("Finishing up")
        finish_time = time()
        this_node_metric_dir = args.output / "__meta__" / hostname
        this_node_metric_dir.mkdir(exist_ok=True, parents=True)

        logger.debug("Writing runtime metrics")
        with (this_node_metric_dir / "last_run.txt").open("w") as f:
            print(finish_time, file=f)

        with (this_node_metric_dir / "run_times.txt").open("a") as f:
            print(f"{start_time},{finish_time - start_time}", file=f)

        if args.git:
            git("add", "-A")
            git("commit", "-m", f"run report for {hostname}")
            git("pull", "--rebase")
            git("push", always_show=True)

        logger.debug(f"Homedirs processed {pformat(ALL_HOMES)}")
        logger.info("Done!")

        if NEWS_LIST:
            logger.warning("=== IMPORTANT INFORMATION ABOUT THE RUN ===")
            for item in NEWS_LIST:
                logger.warning(f"- {item}")
