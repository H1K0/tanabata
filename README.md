<h1 align="center">🎋 Tanabata Project 🎋</h1>

---

[![Release version][release-shield]][release-link]

## Contents

- [About](#about)
- [Glossary](#glossary)
- [Tanabata library](#tanabata-library)
- [Tanabata DBMS](#tanabata-dbms)
- [Tanabata FM](#tanabata-fm)

## About

Tanabata (_jp._ 七夕) is Japanese festival. People generally celebrate this day by writing wishes, sometimes in the form of poetry, on _tanzaku_ (_jp._ 短冊), small pieces of paper, and hanging them on _sasa_ (_jp._ 笹), bamboo. See [this Wikipedia page](https://en.wikipedia.org/wiki/Tanabata) for more information.

Tanabata Project is a software project that will let you enjoy the Tanabata festival. It allows you to store and organize your data as _sasa_ bamboos, on which you can hang almost any number of _tanzaku_, just like adding tags on it.

## Glossary

**Tanabata (_jp._ 七夕)** is a software package project for storing information and organizing it with tags.

**Sasa (_jp._ 笹)** is a file record. It contains 64-bit ID number, the creation timestamp, and the path to the file.

**Tanzaku (_jp._ 短冊)** is a tag record. It contains 64-bit ID number, creation and last modification timestamps, name and description.

**Kazari (_jp._ 飾り)** is a sasa-tanzaku association record. It contains the creation timestamp and associated sasa and tanzaku IDs.

**Hyou (_jp._ 表)** is a table.

**Sasahyou (_jp._ 笹表)** is a table of sasa.

**Sappyou (_jp._ 冊表)** is a table of tanzaku.

**Shoppyou (_jp._ 飾表)** is a table of kazari.

**TDB (Tanabata DataBase)** is a relational database that consists of three tables: _sasahyou_, _sappyou_ and _shoppyou_.

**TDBMS (Tanabata DataBase Management System)** is a management system for TDBs.

**TFM (Tanabata File Manager)** is a TDBMS-powered file manager.

**Tweb (Tanabata web)** is the web user interface for TDBMS and TFM.

## Tanabata library

Tanabata library is a C library for TDB operations. Documentation coming soon...

## Tanabata DBMS

Tanabata Database Management System is the management system for Tanabata databases. Documentation coming soon...

## Tanabata FM

Tanabata File Manager is the TDBMS-powered file manager. Full documentation is [here](docs/fm.md).

---

<h6 align="center"><i>&copy; Masahiko AMANO aka H1K0, 2022—present</i></h6>

[release-shield]: https://img.shields.io/github/release/H1K0/tanabata/all.svg?style=for-the-badge
[release-link]: https://github.com/H1K0/tanabata/releases
