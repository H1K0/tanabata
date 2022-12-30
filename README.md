<h1 align="center">Tanabata file manager</h1>

---

[![Release version][release-shield]][release-link]

## Contents

- [About](#about)
- [Glossary](#glossary)
- [Usage](#usage)
  - [Command Line Interface](#command-line-interface)

## About

Tanabata (_jp._ 七夕) is a Japanese festival. People generally celebrate this day by writing wishes, sometimes in the form of poetry, on _tanzaku_ (_jp._ 短冊), small pieces of paper, and hanging them on _sasa_ (_jp._ 笹), bamboo. See [this Wikipedia page](https://en.wikipedia.org/wiki/Tanabata) for more information.

Tanabata FM is a file manager for Linux that will let you enjoy the Tanabata festival. It organizes files as _sasa_ bamboos, on which you can hang almost any number of _tanzaku_, just like adding tags on it.

## Glossary

**TFM (Tanabata File Manager)** is this file manager.

**Sasa (_jp._ 笹)** is a file record. It contains 64-bit ID number, the creation timestamp, and the path to the file.

**Tanzaku (_jp._ 短冊)** is a tag record. It contains 64-bit ID number, creation and last modification timestamps, name and description.

**Kazari (_jp._ 飾り)** is a sasa-tanzaku association record. It contains the creation timestamp and associated sasa and tanzaku IDs.

**Hyou (_jp._ 表)** is a table or database.

**Sasahyou (_jp._ 笹表)** is a database of sasa.

**Sappyou (_jp._ 冊表)** is a database of tanzaku.

**Shoppyou (_jp._ 飾表)** is a database of kazari.

## Usage

First of all, compile the source code with `./build.sh`. By default, it builds all targets to the `./build/` directory, but you can specify your custom build directory and target with `./build.sh -b <build_dir> -t <target>`. For example, if you want to build just the CLI app to the `./tfm-cli/` directory, run `./build.sh -t tfm -b ./tfm-cli/`.

### Command Line Interface

Build the CLI app using `./build.sh -t tfm [-b <build_dir>]`. For better experience, you can move the CLI executable to the `/usr/bin/` directory (totally safe unless you have another app named `tfm`) or add the directory with it to `PATH`.

Then just open the terminal and run `tfm -h`. If you are running it for the first time, run it with `sudo` or manually create the `/etc/tfm/` directory and check its permissions. This is the directory where TFM stores its config file. If everything is set up properly, you should get the following.

```
(C) Masahiko AMANO aka H1K0, 2022—present
(https://github.com/H1K0/tanabata)

Usage:
tfm <options>

Options:
-h                         Print this help and exit
-I <dir>                   Initialize new Tanabata database in directory <dir>
-O <dir>                   Open existing Tanabata database from directory <dir>
-i                         View database info
-s                         Set or add
-u                         Unset or remove
-e                         Edit or update
-f <sasa_id or path>       File-sasa menu
-t <tanzaku_id or name>    Tanzaku menu
-c <sasa_id>-<tanzaku_id>  Kazari menu (can only be used with the '-s' or '-u' option)
-w                         Weed (defragment) database
-V                         Print version and exit

No database connected
```

So, let's take a look at each option.

Using the `-I <dir>` option, you can initialize the TFM database in the specified directory. The app creates empty sasahyou, sappyou and shoppyou files and saves the directory path to a configuration file. The new database will be used the next time you run the app until you change it.

Using the `-O <dir>` option, you can open the TFM database in the specified directory. The app checks if the directory contains the sasahyou, sappyou and shoppyou files, and if they exist and are valid, saves the directory path to a configuration file. The new database will be used the next time you run the app until you change it.

Using the `-i` option, you can get info about your database. When your hyous were created and last modified, how many records and holes they have, and so on.

Using the `-s` option, you can add new sasa, tanzaku, or kazari.

Using the `-u` option, you can remove sasa, tanzaku, or kazari.

Using the `-e` option, you can update sasa file path or tanzaku name or description. If you want to keep the current value of a field (for example, if you want to change the description of tanzaku while keeping its name), just leave its line blank.

Using the `-f` option, you can manage your sasa. It takes sasa ID when used alone or with the `-u` or `-e` option or target file path when used with the `-s` option. If you want to view the list of all sasa, pass `.` as an argument. For example, `tfm -f 2d` prints the info about sasa with ID `2d` and `tfm -sf path/to/file` adds a new file to the database.

Using the `-t` option, you can manage your tanzaku. It takes tanzaku ID when used alone or with the `-u` or `-e` option or the name of new tanzaku when used with the `-s` option. If you want to view the list of all tanzaku, pass `.` as an argument. For example, `tfm -t c4` prints the info about sasa with ID `c4` and `tfm -st "New tag name"` adds a new tanzaku to the database.

The `-c` option can be used only with the `-s` or `-u` option. It takes the IDs of sasa and tanzaku to link/unlink separated with a hyphen. For example, `tfm -sc 10-4d` links sasa with ID `10` and tanzaku with ID `4d`.

Using the `-w` option, you can _weed_ the database. It's like defragmentation. For example, if you had 4 files with sasa IDs 0, 1, 2, 3 in your database and removed the 1st one, then your database would only have sasa IDs 0, 2, 3 and ID 1 would be a _hole_. Weeding fixes this hole by changing sasa ID 2 to 1, 3 to 2, and updating all associated kazari, so for large databases this can take a while.

Using the `-V` option, you just get the current version of TFM.

---

<h6 align="center"><i>&copy; Masahiko AMANO aka H1K0, 2022—present</i></h6>

[release-shield]: https://img.shields.io/github/release/H1K0/tanabata/all.svg?style=for-the-badge
[release-link]: https://github.com/H1K0/tanabata/releases
