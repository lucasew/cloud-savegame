# cloud-savegame

Steam cloud save is kinda cool but not every possible game supports it and there is no simple way to workaround this.

This is my attempt to solve that.

This tool finds save files defined as rules that tell where to look for the savegames. Not only savegames are supported but basically any software state.

It copies the files to the output folder by game name and grouping.

A configuration file is required to use the program. An example one is provided in the repo and was used to test the software.

No Windows support is planned although it should work the same way because we don't depend on specific platform stuff (pathlib is multiplatform).

This tool is in early development with the hope to be useful, at least for me. **I am not responsible if your backup fails for some reason**.

## How to use
- Install Python
    - 3.10 or above is recommended
    - Should work above 3.8
    - Not sure if 3.5 has all the stuff it  use, but should work
    - Python incompatibilites should be obvios (like give you a missing import error)
- Git (optional)
    - If you want repo syncing this is required
- Run the backup.py script using Python
    - `--help` will give you all information you need
