#!/usr/bin/env python3

from pathlib import Path
from argparse import ArgumentParser, ArgumentDefaultsHelpFormatter
from configparser import ConfigParser
from pprint import pformat
import os
import re
import sys
from shutil import which
import subprocess
import itertools
from time import time
from typing import Optional, Dict, List
from collections import defaultdict
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

config = ConfigParser()
config['general'] = {}
config['general']['divider'] = ','

DEFAULT_CONFIG_FILE = Path(__file__).parents[0] / "demo.cfg"

HOMEFINDER_FIND_FOLDERS = [ ".config", "AppData" ]
HOMEFINDER_IGNORE_FOLDERS = ["dosdevices", "nixpkgs", ".git", ".cache"]
HOMEFINDER_DOCUMENTS_FOLDER = [ "Documentos", "Documents" ];

parser = ArgumentParser(
    formatter_class=ArgumentDefaultsHelpFormatter,
    prog='cloud-savegame',
    description='Backs up games saved data'
)

parser.add_argument('-c', '--config', type=Path, help="Configuration file to be used by the application", default=DEFAULT_CONFIG_FILE)
parser.add_argument('-o', '--output', type=Path, help="Which folder to copy backed up files", required=True)
parser.add_argument('-v', '--verbose', help="Give more detail about what is happening", action='store_true')
parser.add_argument('-g', '--git', help="Use git for snapshot", action='store_true')
parser.add_argument('-b', '--backlink', help="Create symlinks at the origin pointing to the repo", action='store_true')
parser.add_argument('--max-depth', dest="max_depth", help="Max depth for filesystem searches", type=int, default=10)

args = parser.parse_args()

assert args.config.is_file(), "Configuration file is not actually a file"
assert args.output.is_dir() or not args.output.exists(), "Output folder is not actually a folder"
if not args.output.exists():
    args.output.mkdir(exist_ok=True, parents=True)

if args.verbose:
    logging.root.setLevel(logging.DEBUG)

logger.debug("loading configuration file")
config.read(args.config)

NEWS_LIST = []
def warning_news(message: str):
    NEWS_LIST.append(message)
    logger.warning(message)

"""
Config file helpers
"""
def get_str(section: str, key: str) -> Optional[str]:
    if not section in config:
        return None
    if not key in config[section]:
        return None
    return config[section][key]

def get_list(section: str, key: str) -> Optional[List[str]]:
    divider = get_str('general', 'divider')
    raw = get_str(section, key) or ''
    raw = raw.strip()
    if len(raw) == 0:
        return None
    return list(raw.split(divider))


def get_paths(section: str, key: str):
    ret = []
    for p in get_list(section, key) or []:
        ret.append(Path(os.path.expanduser(p)).resolve())
    return set(ret)

def get_bool(section: str, key: str):
    return get_str(section, key) is not None

def get_hostname() -> str:
    """
    Get hostname of this machine to report it in the commit
    """
    import socket
    return socket.gethostname()
hostname = get_hostname()

def delete(item: Path):
    """
    Delete either a file or a folder
    """
    from shutil import rmtree
    item = Path(item)
    item_new = item.parent / f"REMOVE.{item.name}"
    item.rename(item_new)
    logger.info(f"rm: {item}")
    warning_news(f"rm: removed item '{item}'. It was moved to {item_new}.")
    if item_new.is_dir():
        rmtree(str(item_new))
    else:
        item_new.unlink(missing_ok=True)

ignored_paths = get_paths('search', 'ignore')

# print(args)
# print(config)

logger.debug("parsed config file:")
logger.debug({section: dict(config[section]) for section in config.sections()})

git_bin = which("git")

def git(*params, always_show=False) -> None:
    """
    Run git with the parameters if it's enabled

    Noop if git is disabled
    """
    if args.git:
        assert git_bin is not None, "git is not installed"
        kwargs=dict()
        if not (args.verbose or always_show):
            kwargs['stdout'] = subprocess.DEVNULL
            kwargs['stderr'] = subprocess.DEVNULL
        logger.info("git: %s" %(" ".join(map(lambda p: f"'{p}'", params))))
        subprocess.call([git_bin, *params], **kwargs)

def git_is_repo_dirty() -> bool:
    """
    Is the Git repo with uncommited files?
    """
    status_result = subprocess.run(['git', 'status', '-s'], capture_output=True, text=True)
    assert status_result.stdout is not None
    return len(status_result.stdout) > 0

start_time = time()

os.chdir(str(args.output))

if args.git:
    from subprocess import Popen
    if not (args.output / ".git").exists():
        git("init", "--initial-branch", "master")
    is_repo_initially_dirty = git_is_repo_dirty()
    if is_repo_initially_dirty:
        git("add", "-A")
        git("stash", "push")
    if is_repo_initially_dirty:
        git("stash", "pop")
        git("add", "-A")
        git("commit", "-m", f"dirty repo state from hostname {hostname}")

RULES_DIR = [Path(__file__).parents[0] / "rules", args.output / "__rules__"]
RULES_DIR[1].mkdir(exist_ok=True, parents=True)

apps = set()
required_vars = defaultdict(lambda: set())
var_users = defaultdict(lambda: set())
all_vars = set()
rulefiles = {}

def parse_rules(app: str):
    """
    Parse rules from one app
    """
    rulefile = rulefiles[app]
    logger.debug(f"loading rule '{rulefile}'")
    for line in Path(rulefile).read_text().split('\n'):
        rule = line.strip()
        if len(rule) > 0:
            parts = rule.split(' ')
            rule_name = parts[0]
            if get_bool(app, f"ignore_{rule_name}"):
                continue
            rule_path = " ".join(parts[1:])
            # print('rule', rule_name, rule_path)
            yield rule_name.strip(), rule_path.strip()

# load rules
rules_amount = 0

for ruledir in RULES_DIR:
    if not ruledir.is_dir():
        continue
    for rulefile in ruledir.glob('*.txt'):
        appname = rulefile.stem
        apps.add(appname)
        rulefiles[appname] = rulefile

        for rule_name, rule_path in parse_rules(appname):
            variables = list(re.match('\$([a-z]*)', rule_path).groups())
            if len(variables) == 0:
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

def copy_item(input_item, destination, depth=0):
    """
    Copy either a file or a folder from source to destination
    """
    from shutil import copyfile, SameFileError
    input_item = Path(input_item.resolve())
    destination = Path(destination.resolve())
    print('item', input_item, destination)
    if not input_item.exists():
        return
    if str(input_item).startswith(str(args.output)):
        logger.warning((" "*depth) + f"copy_item: Not copying '{input_item}': Origin is inside output")
        return
    if input_item.is_symlink():
        logger.warning((" "*depth, + f"copy_item: Not copying '{input_item}' because it's a symlink"))
        return
    if input_item.is_file():
        destination.parent.mkdir(exist_ok=True, parents=True)
        if destination.exists():
            if (input_item.stat().st_mtime < destination.stat().st_mtime):
                if args.verbose:
                    logger.debug((""*depth) + f"copy_item: Not copying '{input_item}': Didn't change")
                return
        logger.info((" "*depth) + f"copy_item: Copying '{input_item}' to '{destination}'")
        try:
            copyfile(input_item, destination)
        except SameFileError:
            pass
    if input_item.is_dir():
        destination.mkdir(exist_ok=True, parents=True)
        for item in map(lambda x: x.name, input_item.iterdir()):
            copy_item(input_item / item, destination / item, depth=depth+1)

def is_path_ignored(path) -> bool:
    """
    Is the path in the list of ignored paths?
    """
    for ignored in ignored_paths:
        if str(path).startswith(str(ignored)):
            logger.info(f"copy_item: Path ignored: {path}")
            return True
    return False



def ingest_path(app: str, rule_name: str, path: str, top_level=False):
    """
    Ingest a path for an app and rulename

    top_level is a strategy to keep track of items for the backlink feature
    """
    if is_path_ignored(path):
        return
    path = str(path)
    ppath = Path(path)
    output_dir = args.output / app / rule_name
    if not output_dir.exists():
        output_dir.mkdir(exist_ok=True, parents=True)
    if "*" in path:
        top_level = False
        filename = ppath.name
        parent = ppath.parent
        assert "*" not in str(parent), f"globs in any path segment but the last are unsupported. This is a rule bug. app={app} rule_name={rule_name} path='{path}'"
        names = set([x.name for x in [*parent.glob(filename), *output_dir.glob(filename)]])
        for name in names:
            item = parent / name
            new_rule_name = rule_name
            if item.is_dir():
                new_rule_name = str(Path(new_rule_name) / item.name)
            ingest_path(app, new_rule_name, parent / name, top_level=True)
    elif ppath.exists():
        logger.info(f"ingest '{str(path)}' '{str(output_dir)}'")
        copy_item(ppath, output_dir)
        if args.git:
            if git_is_repo_dirty():
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
            delete(ppath)
        # if output_dir.name != ppath.name:
        #     output_dir = output_dir / ppath.name
        logger.info(f"ln {ppath} -> {output_dir}")
        ppath.symlink_to(output_dir)
    if ppath.is_symlink() and not ppath.exists():
        warning_news(f"This may be a rule or a program bug: '{ppath}' points to a non existent location.")

for game in var_users['installdir']:
    game_install_dirs = get_paths(game, 'installdir')
    if game_install_dirs is None:
        if get_str(game, 'not_installed') is None:
            warning_news(f"installdir missing for game {game}, please add it in the game configuration section or set anything to not_installed to disable this warning")
        continue
    for game_install_dir in game_install_dirs:
        if not game_install_dir.exists():
            warning_news(f"Game install dir for {game} doesn't exist: {game_install_dir}")
            continue
        if is_path_ignored(game_install_dir):
            continue
        for rule_name, rule_path in parse_rules(game):
            resolved_rule_path = rule_path.replace('$installdir', str(game_install_dir.resolve()))
            if rule_path == resolved_rule_path:
                continue
            ingest_path(game, rule_name, resolved_rule_path, top_level=True)

def search_for_homes(
        start_dir,
        max_depth=args.max_depth
):
    """
    Return an iterator of home dirs found starting from one start_dir
    """
    if max_depth <= 0:
        return
    if start_dir.is_symlink():
        return
    if not start_dir.is_dir():
        return
    if is_path_ignored(start_dir):
        return
    if start_dir.name in HOMEFINDER_IGNORE_FOLDERS:
        return
    try:
        for pattern in HOMEFINDER_FIND_FOLDERS:
            if (start_dir / pattern).exists():
                yield start_dir
                break
        for item in start_dir.iterdir():
            for home in search_for_homes(item, max_depth=max_depth - 1):
                yield home
    except PermissionError:
        return


def get_homes():
    """
    Get all homes using data from the config file and search_for_homes
    """
    extra_homes = get_paths('search', 'extra_homes')
    if extra_homes is not None:
        for home in extra_homes:
            if is_path_ignored(home):
                continue
            if not home.exists():
                warning_news(f"extra home '{str(home)}' does not exist")
            else:
                yield home
    for search_path in get_paths('search', 'paths'):
        for home in search_for_homes(search_path):
            yield home

ALL_HOMES = []
for homedir in get_homes():
    if is_path_ignored(homedir):
        continue
    ALL_HOMES.append(homedir)
    logger.debug(f"Looking for stuff in {str(homedir)}")
    for game in var_users.get('home') or []:
        for rule_name, rule_path in parse_rules(game):
            resolved_rule_path = rule_path.replace('$home', str(homedir))
            if rule_path == resolved_rule_path:
                continue
            ingest_path(game, rule_name, resolved_rule_path, top_level=True)

    for game in var_users['appdata']:
        appdata = homedir / "AppData"
        for rule_name, rule_path in parse_rules(game):
            resolved_rule_path = rule_path.replace('$appdata', str(appdata))
            if rule_path == resolved_rule_path:
                continue
            ingest_path(game, rule_name, resolved_rule_path, top_level=True)

    has_program_files = False
    for program_files_candidate in homedir.parent.parent.iterdir():
        try:
            if not (program_files_candidate / "Common Files").exists():
                continue
            has_program_files = True
            program_files = program_files_candidate
            for game in var_users['program_files']:
                for rule_name, rule_path in parse_rules(game):
                    resolved_rule_path = rule_path.replace('$program_files', str(program_files))
                    if rule_path == resolved_rule_path:
                        continue
                    ingest_path(game, rule_name, resolved_rule_path, top_level=True)

            ubisoft_users = set()
            ubisoft_specific_dir = args.output / "ubisoft"
            ubisoft_specific_dir.mkdir(parents=True, exist_ok=True)
            ubisoft_users_file = ubisoft_specific_dir / "users.txt"
            if ubisoft_users_file.exists():
                for user in ubisoft_users_file.read_text().strip().split("\n"):
                    ubisoft_users.add(user)
            ubisoft_savegame_dir = program_files / "Ubisoft" / "Ubisoft Game Launcher" / "savegames"
            if ubisoft_savegame_dir.exists():
                for ubisoft_user in ubisoft_savegame_dir.iterdir():
                    if ubisoft_user.is_dir():
                        logger.debug(f"UBISOFT/iterdir: {ubisoft_user}")
                        ubisoft_users.add(ubisoft_user.name)
            ubisoft_users_file.write_text("\n".join(list(ubisoft_users)))

            for ubisoft_user in ubisoft_users:
                ubisoft_var = ubisoft_savegame_dir / ubisoft_user
                for game in var_users['ubisoft']:
                    for rule_name, rule_path in parse_rules(game):
                        resolved_rule_path = rule_path.replace("$ubisoft", str(ubisoft_var))
                        if rule_path == resolved_rule_path:
                            continue
                        logger.debug(f"UBISOFT {resolved_rule_path} {ubisoft_users}")
                        ingest_path(game, rule_name, resolved_rule_path, top_level=True)
        except PermissionError:
            has_program_files = True
            continue
        if not has_program_files:
            warning_news(f"home '{homedir}' is neither a Linux home nor has a Windows-like Program Files. This is a bug and a proof that this implementation is incomplete. Please report. Context: https://twitter.com/lucas59356/status/1700965748611449086.")

    for documents_candidate in HOMEFINDER_DOCUMENTS_FOLDER:
        documents = homedir / documents_candidate
        if not documents.exists():
            continue
        for game in var_users['documents']:
            for rule_name, rule_path in parse_rules(game):
                resolved_rule_path = rule_path.replace('$documents', str(documents))
                if rule_path == resolved_rule_path:
                    continue
                ingest_path(game, rule_name, resolved_rule_path, top_level=True)

finish_time = time()
this_node_metric_dir = args.output / "__meta__" / hostname
this_node_metric_dir.mkdir(exist_ok=True, parents=True)

logger.debug("Writing runtime metrics")
with (this_node_metric_dir / "last_run.txt").open('w') as f:
    print(finish_time, file=f)

with (this_node_metric_dir / "run_times.txt").open('a') as f:
    print(f"{start_time},{finish_time - start_time}", file=f)

git("add", "-A")
git("commit", "-m", f"run report for {hostname}")

git("pull", "--rebase")
git("push", always_show=True)
logger.debug(f"Homedirs processed {pformat(ALL_HOMES)}")
logger.info("Done!")

if len(NEWS_LIST) > 0:
    logger.warning("=== IMPORTANT INFORMATION ABOUT THE RUN ===")
    for item in NEWS_LIST:
        logger.warning(f"- {item}")
