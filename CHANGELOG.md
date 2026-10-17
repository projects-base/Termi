# Changelog

What changed in each release, in the words of someone using Termi rather than
someone reading the diff.

Termi shows the entry for a version inside the app: the first launch after an
update opens **What's new**, and the update bar can show a version's entry
before you install it. That makes this file part of the product, so an entry is
worth writing for what it changes for the person using it, not for the
implementation.

The heading for a release has to be `## <version>`, three-part and with no `v`,
or Termi will not find it and falls back to the commit subjects for that range.

## 7.5.0 — 2026-10-17

### Added

- **What's new**, in the app. The first launch after an update shows what
  changed in the version you just got, so an update that installed itself does
  not land silently. The update bar can also show a version's entry before you
  install it, and Settings has a **What's new** link to read it again later.
- This changelog.

### Fixed

- **A setting could vanish when two Termi windows were open.** Settings live in
  one file, and each window wrote the whole of it from its own copy — so a
  window that had been open a while would put the file back to how it looked
  when it started, undoing anything saved since. Installing a release made this
  likely, because the installer offers to open a second window. Every setting is
  now saved by reading the file, changing the one thing, and writing it back.
  The **Startup Folder** was the most visible casualty.
- **The Settings panel can no longer blank settings it does not show**, such as
  a skipped version.
- **Settings are written through a temporary file and a rename**, so an
  interrupted write cannot leave half a file behind for the next launch.
- **The installer no longer starts Termi as administrator.** The installer needs
  elevation, and anything it launched inherited it — so the terminal it opened
  for you ran every command elevated, with nothing to show that it had.
- **Installing while Termi is running now says so.** Windows will not replace a
  running program, so that install would quietly leave the old one in place.

## 7.3.0 — 2026-10-17

### Added

- **A new version tells you about itself.** A few seconds after launch, if
  GitHub has a newer release, a bar appears above the tabs naming it, the
  version you are on and how long ago it was released. It stays until you act
  on it rather than fading away unseen.
- **Updating without leaving Termi.** That bar downloads the installer, checks
  it against the checksums published with the release, and installs it. Termi
  closes, Windows asks for permission, and the new version opens. Downloading
  and installing are separate clicks, because installing closes the terminals
  you are working in.
- **Skipping a version.** Told about a release you do not want, you can silence
  that one and still hear about anything newer. Reversible from Settings.
- **Turning it off.** **Update notice** in Settings stops the check entirely,
  which also stops Termi contacting GitHub on its own. The **Check for updates**
  button still works whenever you want to look.

### Fixed

- **The installer no longer stacks a duplicate PATH entry on every install.**
  Three installs had left three identical entries. Installing over an existing
  copy now leaves exactly one.
- **Uninstalling no longer corrupts a similarly named folder on your PATH.**
  Removing `...\Termi` used to chew into a neighbouring `...\Termi Beta`,
  because the match was on the folder name rather than on a whole PATH entry.
- An upgrade now installs on top of the existing copy even when it was
  installed somewhere other than the default folder.

## 7.1.0 — 2026-10-17

### Fixed

- **Pasting a screenshot into the command bar works.** It did nothing before:
  the pasted data was read after an `await`, by which point the browser had
  already emptied it. Pasting a file copied in Explorer was unaffected, which
  is why only images seemed broken. A paste that cannot be turned into an
  attachment now says so instead of failing silently.

## 7.0.0 — 2026-10-17

### Added

- **Attach a file to the command bar** by dragging it in from Explorer or
  pasting it, and its path is inserted, quoted if it needs to be.
- **A preview for what you attached**, so you can see you picked the right
  file before running anything: a thumbnail for images, the first rows for CSV
  and Excel files, the first lines for text.
- Pasted screenshots, which have no file of their own, are saved into
  `%APPDATA%\termi\attachments\` and attached from there.

## 6.2.0 — 2026-10-14

### Fixed

- The release build no longer searches the whole disk for NSIS on every run,
  which had been adding minutes to it.

## 6.1.0 — 2026-10-14

### Fixed

- The release build finds the installer compiler wherever Chocolatey puts it,
  and fails with a clear message when it is genuinely missing instead of
  carrying on and failing later for no visible reason.

## 6.0.0 — 2026-10-14

### Added

- **The running version is visible** in the title bar and in Settings, with a
  **Check for updates** button next to it.
- **A startup folder setting**, so new terminals open where you work rather
  than in the install folder.

### Fixed

- The release build survives the intermittent Go module and checksum server
  failures that had been breaking it, and builds the frontend before running
  the tests so a clean checkout can compile at all.

## 5.0.0 — 2026-09-19

### Added

- The release build checks that the tag and the version in `wails.json` agree
  before building anything.

## 4.0.0 — 2026-09-19

### Added

- Building the installer in CI, and the documentation for it.

### Fixed

- The installer build script.

## 3.0.0 — 2026-04-07

The first tagged release.

---

Entries for 5.0.0 and earlier are summarised from the commit history rather
than written at the time, so they are shorter than the ones above.
