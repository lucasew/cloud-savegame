from pathlib import Path
from argparse import ArgumentParser
from configparser import ConfigParser
from pprint import pprint
import os
import re
from distutils import dir_util, file_util

config = ConfigParser()
config['general'] = {}
config['general']['divider'] = ','

parser = ArgumentParser(prog='gamebkp', description='Backs up games saved data')

parser.add_argument('-c', '--config', type=Path, help="Configuration file to be used by the application", default=Path(__file__).parents[0] / "demo.cfg")
parser.add_argument('-o', '--output', type=Path, help="Which folder to copy backed up files", required=True)


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

def get_paths(section: str, key: str):
    ret = []
    divider = get_str('general', 'divider')
    raw = get_str(section, key) or ''
    raw = raw.strip()
    if len(raw) == 0:
        return None
    for p in raw.split(divider):
        ret.append(Path(os.path.expanduser(p)).resolve())
    return ret

print(args)
print(config)
pprint({section: dict(config[section]) for section in config.sections()})

print(get_paths('eoq', 'trabson'))
print(get_paths('search', 'paths'))

apps = set()
required_vars = {}
var_users = {}
all_vars = set()

def parse_rules(app: str):
    with (Path(__file__).parents[0] / "rules" / f"{app}.txt").open() as f:
        rule = f.readline().strip()
        if len(rule) > 0:
            parts = rule.split(' ')
            rule_name = parts[0]
            rule_path = " ".join(parts[1:])
            print('rule', rule_name, rule_path)
            yield rule_name, rule_path

# load rules
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

pprint(apps)
pprint(required_vars)
pprint(all_vars)
pprint(var_users)

def ingest_path(app: str, rule_name: str, path: Path):
    ppath = Path(path)
    output_dir = args.output / app / rule_name
    output_dir.mkdir(exist_ok=True, parents=True)
    if ppath.exists():
        if ppath.is_dir():
            print("Copying folder ", str(path), "...")
            dir_util.copy_tree(str(path), str(output_dir), update=1, verbose=1)
        else:
            print("Copying file ", str(path), "...")
            file_util.copy_file(str(path), str(output_dir), update=1, verbose=1)

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

for search_path in get_paths('search', 'paths'):
    for appdata in search_path.glob('**/AppData'):
        homedir = appdata.parents[0]
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

