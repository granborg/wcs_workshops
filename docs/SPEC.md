# WCS Workshops: Product Specification

Mobile-first web app in Vue 3 (`<script setup lang="ts">`), wrapped in Capacitor for iOS and Android.
Backed by Vimeo as the video hosting service with domain-level privacy settings.
Users log on with OAuth/SSO (Google, Apple, Microsoft).

There are two user types: **viewers**, who view videos, and **creators**, who upload and share videos with
viewers and other creators.

## Library (Books icon)

The main page lists every creator who has shared with the user. Opening a creator shows a directory navigator,
similar to Google Drive.

If the user is a creator, their own collection appears in bold and they have administrative options for every
video and directory in it.

### Per directory and per video (creator only)

**Share.** Opens a dialog with QR code creation. If a code already exists, the creator can show or print it,
or generate a new one with an optional expiration date and time.

A QR code grants any user who scans it access to the directory or video. The item is added to the user's
collection, organized the same way the creator organized it.

Access to a video follows the video wherever the creator moves it. Access to a directory covers every video in
that directory and its sub-directories.

**View/Edit access.** Shows which users have access and lets the creator revoke any of them. When a user is
granted access, every phone signed in to that account gets a push notification.

Renaming a directory does not change its access list, so every directory has a unique GUID.

**Move.** Any directory or video can be moved. Moving may change who can see it.

**Delete.** Any video can be deleted.

## QR Scan (QR icon)

Scan a code to add a video or directory to the collection. Access lasts until the creator revokes it or removes
the item.

## Settings (Cog icon)

### Account

Accounts have unique, non-user-facing GUIDs. Users can change their name at any time.

- **Account type:** Creator or Viewer. Changing from creator to viewer requires deleting the whole collection.
- **Name:** Viewers see the creator's name in their library. Creators see viewer names when auditing access.

Creators build a folder structure: create, rename, and remove folders. Removing a folder that holds videos asks
whether to delete the videos or move them to another folder.

Creators can upload a banner shown in the library list and while browsing their collection, in place of plain text.

### Audit log

Timestamped: who (which IP) was granted access to which video or directory, which QR code was used, when access
was removed, when a QR code was generated.

## Upload (Upload icon, creator only)

Upload a video and choose its destination folder.
