import os
from configparser import ConfigParser
from pathlib import Path
from typing import List, Optional, Set


def get_str(config: ConfigParser, section: str, key: str) -> Optional[str]:
    """
    Retrieve a string value from the configuration.

    Returns:
        The value if the section and key exist, otherwise None.
    """
    if section not in config or key not in config[section]:
        return None
    return config[section][key]


def get_list(config: ConfigParser, section: str, key: str) -> Optional[List[str]]:
    """
    Retrieve a list of strings from the configuration.

    The value is split using the delimiter defined in `general.divider` (defaulting to comma).
    """
    divider = get_str(config, "general", "divider") or ","
    raw = get_str(config, section, key) or ""
    raw = raw.strip()
    if not raw:
        return None
    return list(raw.split(divider))


def get_paths(config: ConfigParser, section: str, key: str) -> Set[Path]:
    """
    Retrieve a set of Path objects from the configuration.

    It expands user paths (e.g., `~`) and resolves them to absolute paths.
    """
    ret = []
    for p in get_list(config, section, key) or []:
        ret.append(Path(os.path.expanduser(p)).resolve())
    return set(ret)


def get_bool(config: ConfigParser, section: str, key: str) -> bool:
    """
    Check if a key exists in the configuration section.

    This treats the presence of the key as True, and absence as False.
    The actual value of the key is ignored.
    """
    return get_str(config, section, key) is not None
