# cloud-savegame

Steam cloud save is kinda cool but not every possible game supports it and there is no simple way to workaround this.

This is my attempt to solve that.

This tool finds save files with logic defined as simple rules (really, look at the rules folder to see many examples) that tell where to look for the savegames. Not only savegames are supported, but basically any software state. Maybe dotfiles for some specific software?

It also has backlinking as an experimental feature, that in my tests work good enough. With a flag it creates the symlinks in the origin places so when the software writes the file it writes automatically to your saves repository. Issues may include if you move the repo to other location then the symlinks would break. A really cool side effect of this is when you download the same game in more than one app store (including the green steam) and each one creates a Wine/Proton prefix and then your saves are only available in one of the builds of the game. With this feature both games can see each other saves.

It copies the files to the output folder by game name and grouping. Ex: screenshots and mods separated from save games themselves. The ignore logic for git, if, for example there are large mods in there can be done with .gitignore files.

A configuration file is required to use the program. An example one is provided in the repo and was used to test the software.

This tool is in early development with the hope to be useful, at least for me. **I am not responsible if your backup fails for some reason**.

## How to use

- **Install Go** (1.21 or above)
- **Git** (optional, for repo syncing)
- Build the binary:
  ```bash
  go build -o cloud-savegame ./cmd/cloud-savegame
  ```
- Run the tool:
  ```bash
  ./cloud-savegame --help
  ```

## ⚠️ Really important Information about the backlinking feature ⚠️

**By default, the Steam Runtime at most allows read only access for stuff outside the protn prefix and some specific data**

- This means that your Steam Play/Proton game will very likely be able to read the saves but will not be able to write new saves if the canonical destination is the repository.
- This can be solved by setting `STEAM_COMPAT_MOUNTS` in the launch options **for each damn game you will use with this**.
- In my case, I store my saves repo at `/home/lucasew/SavedGames` so a launch option that I use is `STEAM_COMPAT_MOUNTS=/home/lucasew/SavedGames %command%`.
  - (**TODO**: Test if setting STEAM_COMPAT_MOUNTS system wide solves this)
- Related issue: https://github.com/ValveSoftware/steam-runtime/issues/470

## Configuration reference

The configuration follows the INI format.

This means:

- Comments start with `#`.
- No nesting as we have in YAML, it's a map of maps of strings and
  that's it.

For clarification sake, I will reference the options as `section.subsection`.
If I say set `eoq.trabson` to "huehuebrbr" then it would be the following code:

```ini
[eoq]
trabson=huehuebrbr

# yay, ini supports comments
```

There is a simple layer of "typing" over INI primitives.

- String: the normal behaviour one can expect
- List: list of strings, separated by the value of `general.divider`
- Paths: list of paths, separated by the value of `general.divider`. Tilde (`~`) is expanded to user home.
- Boolean: if the value exists (key is present) then it's true, otherwise it's false. To set
  it to false you have to remove/comment it out.

### Core sections

- **String** `general.divider`: What is the divider symbol for each item in a list.
  It's `,` by default. I don't recommend changing this.
- **Paths** `search.extra_homes`: Paths that you are certain that will have home
  directories. May speed up ingestion depending on how you finetune this.
- **Paths** `search.ignore`: Paths that you don't want the program to search.
  I use it for fuse mounts, that will not have relevant stuff anyway
  and are slow to search. Finetuning this can make ingestion a hell of
  fast. Actually that was a game changer for my setup.
- **Paths** `search.paths`: Paths to be searched for home directories.

### What is a home directory in this case?

As you may have deduced, the automatic search system only looks
for home directories. Our definition of home directory is a directory
that has any of the items defined in the top level variable
`HOMEFINDER_FIND_FOLDERS` (currently `.config` and `AppData`). That's it. If it has any of these items
then it's in.

### App specific sections

The name of each app is defined by the rule name. Go search your app or game
in the rules folder to get the exact name.

- **Paths** `$app.installdir`: Required for games, such as Flatout 2, that saves
  their data in the game install directory instead of some user folder.
- **Boolean** `$app.ignore_$output`: In the rule of the game there is a item type,
  like saves, and the path with the special variables. With this flag you can
  avoid that item type to be touched by this tool. I use it for my Farming Simulator
  mods as those are goddamn fat and some of them are over the 100MB hard limit for each
  file in GitHub. Skyrim saves are not so small (~30MB each), but still acceptable.

## Rules reference

Rules are our domain specific language to add support for new apps and games.

We use `filepath` which handles cross-platform path separators.

### Special variables

- `$documents`: The documents folder in a home folder. This may not catch
  your documents folder depending on which language you have set on your PC.
- `$appdata`: The ~/AppData folder in a home folder. In Windows it's hidden by default.
- `$home`: The home folder itself.
- `$installdir`: The folder where the app is installed. All apps that use this must
  have installdir specified in the configuration file for the rule to work.

## FAQ

> Why this exsits?

Steam cloud is cool, works with many games but not all. And with Jack Sparrow games you are on your own ¯\_(ツ)_/¯.
