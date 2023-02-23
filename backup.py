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

config = ConfigParser()
config['general'] = {}
config['general']['divider'] = ','

DEFAULT_CONFIG_FILE = Path(__file__).parents[0] / "demo.cfg"

parser = ArgumentParser(
    formatter_class=ArgumentDefaultsHelpFormatter,
    prog='cloud-savegame',
    description='Backs up games saved data'
)

parser.add_argument('-c', '--config', type=Path, help="Configuration file to be used by the application", default=DEFAULT_CONFIG_FILE)
parser.add_argument('-o', '--output', type=Path, help="Which folder to copy backed up files", required=True)
parser.add_argument('-v', '--verbose', help="Give more detail about what is happening", action='store_true')
parser.add_argument('-g', '--git', help="Use git for snapshot", action='store_true')

args = parser.parse_args()

assert args.config.is_file(), "Configuration file is not actually a file"
assert args.output.is_dir() or not args.output.exists(), "Output folder is not actually a folder"
if not args.output.exists():
    args.output.mkdir(exist_ok=True, parents=True)

config.read(args.config)

def get_str(section: str, key: str):
    if not section in config:
        return None
    if not key in config[section]:
        return None
    return config[section][key]

def get_list(section: str, key: str):
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
    return ret

def get_bool(section: str, key: str):
    return get_str(section, key) is not None

# print(args)
# print(config)

if args.verbose:
    print("parsed config file:")
    pprint({section: dict(config[section]) for section in config.sections()})

git_bin = which("git")
def git(*params, always_show=False):
    if args.git:
        assert git_bin is not None, "git is not installed"
        kwargs=dict()
        if not (args.verbose or always_show):
            kwargs['stdout'] = subprocess.DEVNULL
            kwargs['stderr'] = subprocess.DEVNULL
        print("git: %s" %(" ".join(map(lambda p: f"'{p}'", params))))
        subprocess.call([git_bin, *params], **kwargs)

def git_is_repo_dirty():
    status_result = subprocess.run(['git', 'status', '-s'], capture_output=True, text=True)
    assert status_result.stdout is not None
    return len(status_result.stdout) > 0

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
        git("commit", "-m", "dirty repo state")

apps = set()
required_vars = {}
var_users = {}
all_vars = set()

def parse_rules(app: str):
    for line in (Path(__file__).parents[0] / "rules" / f"{app}.txt").read_text().split('\n'):
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
for rulefile in (Path(__file__).parents[0] / "rules").glob('*.txt'):
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
        copyfile(input_item, destination)
        return
    if input_item.is_dir():
        destination.mkdir(exist_ok=True, parents=True)
        for item in map(lambda x: x.name, input_item.iterdir()):
            copy_item(input_item / item, destination / item, depth=depth+1)


def ingest_path(app: str, rule_name: str, path: str):
    path = str(path)
    ppath = Path(path)
    output_dir = args.output / app / rule_name
    output_dir.mkdir(exist_ok=True, parents=True)
    if "*" in path:
        filename = ppath.name
        parent = ppath.parent
        assert "*" not in str(parent), f"globs in any path segment but the last are unsupported. This is a rule bug. app={app} rule_name={rule_name} path='{path}'"
        if args.verbose:
            print(f"glob ingest path='{path}'")
        for item in parent.glob(filename):
            new_rule_name = rule_name
            if item.is_dir():
                new_rule_name = str(Path(new_rule_name) / item.name)
            ingest_path(app, new_rule_name, item)
    elif ppath.exists():
        if args.verbose:
            print(f"ingest '{str(path)}' '{str(output_dir)}'")
        copy_item(ppath, output_dir)
        if args.git:
            if git_is_repo_dirty():
                commit = f"app={app} rule={rule_name} path={path}"
                git("add", "-A")
                git("commit", "-m", commit)

for game in var_users['installdir']:
    game_install_dirs = get_paths(game, 'installdir')
    if game_install_dirs is None:
        if get_str(game, 'not_installed') is None:
            print(f"installdir missing for game {game}, please add it in the game configuration section or set anything to not_installed to disable this warning")
        continue
    for game_install_dir in game_install_dirs:
        for rule_name, rule_path in parse_rules(game):
            resolved_rule_path = rule_path.replace('$installdir', str(game_install_dir.resolve()))
            if rule_path == resolved_rule_path:
                continue
            ingest_path(game, rule_name, resolved_rule_path)

def get_homes():
    extra_homes = get_paths('search', 'extra_homes')
    if extra_homes is not None:
        for home in extra_homes:
            if not home.exists():
                print(f"Warning: extra home '{str(home)}' does not exist")
            else:
                yield home
    for search_path in get_paths('search', 'paths'):
        for appdata in search_path.glob('**/AppData'):
            yield appdata.parents[0]

for homedir in get_homes():
    if args.verbose:
        print(f"Looking for stuff in {str(homedir)}")
    appdata = homedir / "AppData"
    for game in var_users.get('home') or []:
        for rule_name, rule_path in parse_rules(game):
            resolved_rule_path = rule_path.replace('$home', str(homedir.resolve()))
            if rule_path == resolved_rule_path:
                continue
            ingest_path(game, rule_name, resolved_rule_path)

    for game in var_users['appdata']:
        appdata = homedir / "AppData"
        for rule_name, rule_path in parse_rules(game):
            resolved_rule_path = rule_path.replace('$appdata', str(appdata.resolve()))
            if rule_path == resolved_rule_path:
                continue
            ingest_path(game, rule_name, resolved_rule_path)

    for documents_candidate in [ "Documentos", "Documents" ]:
        documents = homedir / documents_candidate
        if not documents.exists():
            continue
        for game in var_users['documents']:
            for rule_name, rule_path in parse_rules(game):
                resolved_rule_path = rule_path.replace('$documents', str(documents.resolve()))
                if rule_path == resolved_rule_path:
                    continue
                ingest_path(game, rule_name, resolved_rule_path)


git("push", always_show=True)
