<h1 align="center">Tanabata file manager</h1>

---

[![Release version][release-shield]][release-link]

## Contents

- [About](#about)
- [Glossary](#glossary)
- [Usage](#usage)

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

For better experience, you can move the CLI executable to the `/usr/bin/` directory (totally safe unless you have another app named `tfm`) or add the directory with it to `PATH`.

Then just open the terminal and run `tfm -h`. If you are running it for the first time, run it with `sudo` or manually create the `/etc/tfm/` directory and check its permissions. This is the directory where TFM stores its config file. If everything is set up properly, you should get the following.

```
(C) Masahiko AMANO aka H1K0, 2022—present
(https://github.com/H1K0/tanabata)

Usage:
tfm <options>

Options:
-h        Print this help and exit
-I <dir>  Initialize new Tanabata database in directory <dir>
-O <dir>  Open existing Tanabata database from directory <dir>
-i        View database info
-a        View all
-s        Set or add
-u        Unset or remove
-f        File-sasa menu
-t        Tanzaku menu
-k        Kazari menu (can only be used with the '-s' or '-u' option)
-w        Weed (defragment) database
-V        Print version and exit

No database connected
```

So, let's take a look at each option.

Using the `-I <dir>` option, you can initialize the TFM database in the specified directory. The app creates empty sasahyou, sappyou and shoppyou files and saves the directory path to a configuration file. The new database will be used the next time you run the app until you change it.

Using the `-O <dir>` option, you can open the TFM database in the specified directory. The app checks if the directory contains the sasahyou, sappyou and shoppyou files, and if they exist and are valid, saves the directory path to a configuration file. The new database will be used the next time you run the app until you change it.

Using the `-i` option, you can get info about your database. When your hyous were created and last modified, how many records and holes they have, and so on.

Using the `-a` option, you get the full list of what you specify. For example, `-af` prints the full list of sasa.

Using the `-s` option, you can add new sasa, tanzaku, or kazari. For example, `-st` launches a menu for adding new tanzaku. Just enter its name and description.

Using the `-u` option, you can remove sasa, tanzaku, or kazari. For example, `-uk` launches a menu for removing kazari. Just enter the sasa and tanzaku IDs to dissociate them.

Using the `-f` option without others, you can view the info about a specific sasa. Just enter the sasa ID.

Using the `-t` option without others, you can view the info about a specific tanzaku. Just enter the tanzaku ID.

The `-k` option can be used only with the `-s` or `-u` option.

Using the `-w` option, you can _weed_ the database. It's like defragmentation. For example, if you had 4 files with sasa IDs 0, 1, 2, 3 in your database and you removed the 1st one, then your database would only have sasa IDs 0, 2, 3 and ID 1 would be a _hole_. Weeding fixes this hole by changing sasa ID 2 to 1, 3 to 2, and updating all associated kazari, so for large databases this can take a while.

Using the `-V` option, you just get the current version of TFM.

---

<h6 align="center"><i>&copy; Masahiko AMANO aka H1K0, 2022—present</i></h6>

[release-shield]: https://img.shields.io/github/release/H1K0/tanabata/all.svg?style=for-the-badge
[release-link]: https://github.com/H1K0/tanabata/releases
