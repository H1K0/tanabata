# Tanabata File Manager — Requirements

> Product requirements for Tanabata File Manager (TFM). Architecture and code
> layout are described separately in [ARCHITECTURE.md](ARCHITECTURE.md),
> [GO_PROJECT_STRUCTURE.md](GO_PROJECT_STRUCTURE.md) and
> [FRONTEND_STRUCTURE.md](FRONTEND_STRUCTURE.md).

## Overview

Tanabata File Manager (TFM) is a multi-user, tag-based web file manager. It runs
on a client–server architecture and is operated entirely through a web
interface. Its goal is centralized, server-side storage of files with access and
management from both desktop and mobile browsers. The application is primarily
oriented toward **images and video**.

The app is a PWA that can be installed on a desktop or a phone. Files managed by
Tanabata are stored flat in a single directory; each file's on-disk name equals
its UUID in the database.

Support for additional database engines is planned for future versions.

## Core Concepts

- **File** — a single file on the server. It may carry any number of tags and
  belong to any number of pools. It has a creator and optional access settings
  (a user — which may be null, making the file public — plus read and edit
  permission flags). It has an original name and metadata (key–value, including
  all EXIF data).
- **Tag** — a label on a file. It may be attached to any number of files and
  belong to at most one category. It has a name, a description, key–value
  metadata, and may define auto-tag rules.
- **Auto-tag (tag rule)** — a rule stating that when tag A is attached to a file,
  tag B is attached to the same file automatically.
- **Category** — an entity that logically groups several tags. It has a name, a
  description, and key–value metadata.
- **Pool** — a logical grouping of files. It has a name, a description, and
  key–value metadata. Files in a pool can be sorted automatically or arranged in
  a user-defined manual order.

## Functional Requirements

### 1. File management

1. Browse the file list (lazy load, pagination).
2. Filter files by tags and metadata.
3. View and edit sort settings (persisted per user).
4. Multi-select files (Ctrl, Shift) and act on the selection:
   1. Attach / detach tags.
   2. Copy / paste tags.
   3. Add to a pool.
   4. View and edit access settings.
   5. Delete (with a confirmation prompt).
5. View a single file.
6. Single-file actions:
   1. Attach / detach tags.
   2. Copy / paste tags.
   3. Add to a pool.
   4. View and edit access settings.
   5. Replace the file (upload new content under the same ID).
   6. Delete (with a confirmation prompt).
7. Browse files gallery-style (prev/next paging through the viewer).
8. Upload new files through the web UI (form or drag-and-drop onto the list).
9. Import new files from a folder on the server.
10. Near-duplicate detection for images and video:
    1. Show groups (clusters) of duplicates.
    2. Dismiss false duplicates (the app remembers that file A is _not_ a
       duplicate of file B).
    3. Choose which duplicate to keep and which to delete.
    4. Choose, per field, which duplicate the surviving file inherits it from.
11. Trash:
    1. Browse trashed files.
    2. Restore from trash.
    3. Delete permanently.

### 2. Tag management

1. Browse the tag list (lazy load, pagination).
2. Search by name.
3. View and edit sort settings (persisted per user).
4. Multi-select tags (Ctrl, Shift) and act on the selection:
   1. Assign auto-tag rules.
   2. Change category.
   3. Delete (with a confirmation prompt).
5. View a single tag.
6. Single-tag actions:
   1. Edit name, description, and metadata (key–value).
   2. Change category.
   3. Assign auto-tag rules.
   4. Delete (with a confirmation prompt).
7. Create a tag:
   1. Enter name, description, and metadata (key–value).
   2. Assign a category (optional).
   3. Assign auto-tag rules.

### 3. Category management

1. Browse the category list (lazy load, pagination).
2. Search by name.
3. View and edit sort settings (persisted per user).
4. Multi-select categories (Ctrl, Shift) and act on the selection:
   1. View shared tags and tags attached to some (but not all) of them.
   2. Attach / detach tags.
   3. Delete (with a confirmation prompt).
5. View a single category.
6. Single-category actions:
   1. Edit name, description, and metadata (key–value).
   2. View attached tags.
   3. Attach / detach tags.
   4. Delete (with a confirmation prompt).
7. Create a category:
   1. Enter name, description, and metadata (key–value).
   2. Attach tags.

### 4. Pool management

1. Browse the pool list (lazy load, pagination).
2. Search by name.
3. View and edit sort settings (persisted per user).
4. Multi-select pools (Ctrl, Shift) and act on the selection:
   1. View and edit access settings.
   2. Delete (with a confirmation prompt).
5. View a single pool.
6. Single-pool actions:
   1. Edit name, description, and metadata (key–value).
   2. View and edit access settings.
   3. View all files in the pool.
   4. Filter the pool's files by tags.
   5. Change the file sort setting (including disabling automatic sorting).
   6. Reorder files manually (when automatic sorting is disabled).
   7. Delete (with a confirmation prompt).
7. Create a pool:
   1. Enter name, description, and metadata (key–value).
   2. Attach files.

### 5. User settings

1. Username.
2. Password.
3. Sessions:
   1. Terminate a session.
4. Path to the server folder scanned during file import.

### 6. Server administration (admin panel)

1. Users:
   1. Browse the list.
   2. View a single user.
   3. Create.
   4. Delete.
   5. Block / unblock.
   6. Set role (reader / editor).

### 7. Audit logging (in the database)

Log the following user actions:

1. File views.
2. Changes to file access settings.
3. Create / edit / delete of a file, tag, category, pool, or file–tag relation.
4. Create / block / unblock / delete of a user.
5. User role changes.
6. User login / logout.
7. Session termination.

## Non-Functional Requirements

1. The interface must be as simple and convenient as possible: everything needed
   should be at hand, reachable in the fewest possible actions.
2. The interface must adapt to both desktop and mobile devices.
3. The interface must offer dark and light themes.
4. Use PWA technology, including a button that fully resets the PWA (except the
   cache) and reloads it from the server.
5. Allow selected files to be cached and viewed offline in the installed PWA.
6. First-run setup must require minimal effort: automatic database migration, a
   ready-made Docker Compose file, and a `.env` file with the configurable
   installation parameters.
7. Use a Domain-Driven Design approach on the API server.
8. Reject files whose MIME type is not present in the database (no DB entry — no
   support).
