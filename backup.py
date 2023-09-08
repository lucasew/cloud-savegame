#!/usr/bin/env python3

from pathlib import Path
from argparse import ArgumentParser, ArgumentDefaultsHelpFormatter
from configparser import ConfigParser
from pprint import pprint
import os
import re
import sys
from shutil import which
import subprocess
import itertools
from time import time
from typing import Optional, Dict, List

config = ConfigParser()
config['general'] = {}
config['general']['divider'] = ','

DEFAULT_CONFIG_FILE = Path(__file__).parents[0] / "demo.cfg"
RULES_DIR = Path(__file__).parents[0] / "rules"

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

config.read(args.config)

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
    print("RM: ", item)
    if item_new.is_dir():
        rmtree(str(item_new))
    else:
        item_new.unlink(missing_ok=True)

ignored_paths = get_paths('search', 'ignore')

# print(args)
# print(config)

if args.verbose:
    print("parsed config file:")
    pprint({section: dict(config[section]) for section in config.sections()})

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
        print("git: %s" %(" ".join(map(lambda p: f"'{p}'", params))))
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
    git("pull")
    if is_repo_initially_dirty:
        git("stash", "pop")
        git("add", "-A")
        git("commit", "-m", f"dirty repo state from hostname {hostname}")

apps = set()
required_vars = {}
var_users = {}
all_vars = set()

def parse_rules(app: str):
    """
    Parse rules from one app
    """
    for line in (RULES_DIR / f"{app}.txt").read_text().split('\n'):
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
for rulefile in RULES_DIR.glob('*.txt'):
    appname = rulefile.stem
    required_vars[appname] = set()
    apps.add(appname)

    for rule_name, rule_path in parse_rules(appname):
        variables = list(re.match('\$([a-z]*)', rule_path).groups())
        if len(variables) == 0:
            ingest_path(appname, rule_name, rule_path)
            continue
        for var in variables:
            required_vars[appname].add(var)
            all_vars.add(var)
            if var_users.get(var) is None:
                var_users[var] = set()
            var_users[var].add(appname)
        rules_amount += 1

if args.verbose:
    print(f"loaded {rules_amount} rules for {len(apps)} apps")
    print("all apps with rules loaded: ", apps)
    print("all variables mentioned in rules: ", all_vars)

def copy_item(input_item, destination, depth=0):
    """
    Copy either a file or a folder from source to destination
    """
    from shutil import copyfile
    input_item = Path(input_item)
    destination = Path(destination)
    if not input_item.exists():
        return
    if str(input_item).startswith(str(args.output)):
        if args.verbose:
            print((""*depth) + f"Not copying '{input_item}': Origin is inside output")
        return
    if input_item.is_file() or input_item.is_symlink():
        destination.parent.mkdir(exist_ok=True, parents=True)
        if destination.is_dir():
            destination = destination / input_item.name
        if destination.exists():
            if (input_item.stat().st_mtime < destination.stat().st_mtime):
                if args.verbose:
                    print((""*depth) + f"Not copying '{input_item}': Didn't change")
                return
        print((" "*depth) + f"Copying '{input_item}' to '{destination}'")
        if input_item.is_file():
            copyfile(input_item, destination)
        elif input_item.is_symlink():
            final_path = input_item.resolve()
            if not str(final_path).startswith(str(args.output)):
                print(f"Symlink '{final_path}' doesn't point to a item inside repo path")
        return
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
            print(f"Path ignored: {path}")
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
        if args.verbose:
            print(f"ingest '{str(path)}' '{str(output_dir)}'")
        if not ppath.is_symlink():
            copy_item(ppath, output_dir)
            if args.git:
                if git_is_repo_dirty():
                    commit = f"hostname={hostname} app={app} rule={rule_name} path={path}"
                    git("add", "-A")
                    git("commit", "-m", commit)
        # backlink logic
    if args.backlink and top_level:
        # print(f"TOPLEVEL: {app} {rule_name} {path} {Path(path).resolve()}")
        ppath.parent.mkdir(parents=True, exist_ok=True)
        if ppath.is_symlink():
            ppath.unlink()  # recreate
        elif ppath.exists():
            delete(ppath)
        # if output_dir.name != ppath.name:
        #     output_dir = output_dir / ppath.name
        print(f"ln {ppath} -> {output_dir}")
        ppath.symlink_to(output_dir)

for game in var_users['installdir']:
    game_install_dirs = get_paths(game, 'installdir')
    if game_install_dirs is None:
        if get_str(game, 'not_installed') is None:
            print(f"installdir missing for game {game}, please add it in the game configuration section or set anything to not_installed to disable this warning")
        continue
    for game_install_dir in game_install_dirs:
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
                print(f"Warning: extra home '{str(home)}' does not exist")
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
    if args.verbose:
        print(f"Looking for stuff in {str(homedir)}")
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

with (this_node_metric_dir / "last_run.txt").open('w') as f:
    print(finish_time, file=f)

with (this_node_metric_dir / "run_times.txt").open('a') as f:
    print(f"{start_time},{finish_time - start_time}", file=f)

git("add", "-A")
git("commit", "-m", f"run report for {hostname}")

git("push", always_show=True)
print("Homedirs processed", ALL_HOMES)
print("Done!")
