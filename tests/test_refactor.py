import configparser

from cloud_savegame import parse_rules


def test_parse_rules(tmp_path):
    # Setup
    app_name = "test_app"
    rule_content = """
    save_data $home/.config/test_app
    config $home/.test_app/config
    ignored_rule $home/.ignored
    """
    rule_file = tmp_path / f"{app_name}.txt"
    rule_file.write_text(rule_content)

    rulefiles = {app_name: rule_file}

    config = configparser.ConfigParser()
    config.add_section("general")
    config.add_section(app_name)
    # Ignore 'ignored_rule'
    config.set(app_name, "ignore_ignored_rule", "true")

    # Execute
    results = list(parse_rules(app_name, rulefiles, config))

    # Verify
    expected = [
        ("save_data", "$home/.config/test_app"),
        ("config", "$home/.test_app/config"),
    ]

    # Check that ignored_rule is NOT in results
    assert len(results) == 2
    for item in expected:
        assert item in results
