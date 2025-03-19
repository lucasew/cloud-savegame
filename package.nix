{ python3Packages }:
python3Packages.buildPythonApplication {
  name = "cloud-savegame";
  version = builtins.readFile ./cloud_savegame/VERSION;
  pyproject = true;

  src = ./.;

  build-system = [
    python3Packages.hatchling
  ];

  meta.mainProgram = "cloud_savegame";
}
