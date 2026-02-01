import unittest
from configparser import ConfigParser
from pathlib import Path

from cloud_savegame.config_utils import get_bool, get_list, get_paths, get_str


class TestConfigUtils(unittest.TestCase):
    def setUp(self):
        self.config = ConfigParser()
        self.config["section"] = {
            "key_str": "value",
            "key_list": "item1,item2",
            "key_list_space": "item1, item2",
            "key_bool_present": "anything",
            "key_path": "/tmp/path1,/tmp/path2",
        }
        self.config["general"] = {"divider": ","}

    def test_get_str(self):
        self.assertEqual(get_str(self.config, "section", "key_str"), "value")
        self.assertIsNone(get_str(self.config, "section", "missing"))
        self.assertIsNone(get_str(self.config, "missing", "key_str"))

    def test_get_list(self):
        self.assertEqual(get_list(self.config, "section", "key_list"), ["item1", "item2"])
        # Note: get_list does not strip individual items, only the raw string.
        self.assertEqual(get_list(self.config, "section", "key_list_space"), ["item1", " item2"])
        self.assertIsNone(get_list(self.config, "section", "missing"))

    def test_get_bool(self):
        self.assertTrue(get_bool(self.config, "section", "key_bool_present"))
        self.assertFalse(get_bool(self.config, "section", "missing"))

    def test_get_paths(self):
        # We need to be careful with paths as they are resolved.
        # Just checking if it returns a set of Paths
        paths = get_paths(self.config, "section", "key_path")
        self.assertIsInstance(paths, set)
        self.assertEqual(len(paths), 2)
        self.assertTrue(all(isinstance(p, Path) for p in paths))

        # Test expanduser
        self.config["section"]["home_path"] = "~/test"
        paths = get_paths(self.config, "section", "home_path")
        self.assertEqual(len(paths), 1)

    def test_get_list_custom_divider(self):
        self.config["general"]["divider"] = ";"
        self.config["section"]["key_list_semicolon"] = "item1;item2"
        self.assertEqual(get_list(self.config, "section", "key_list_semicolon"), ["item1", "item2"])
