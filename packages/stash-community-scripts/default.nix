# CommunityScripts — a CURATED SUBSET of the companion plugin collection.
# services.stash.plugins defaults to [] and nixpkgs ships nothing, so a stock
# Stash has no plugins; installing all 79 is the other extreme and worse, since
# per-plugin enable/disable lives in Stash's SQLite. What ships here IS the
# config: adding a plugin is a line in this list, not a click in the UI.
#
# Excluded on purpose:
#
#   * Anything that renames or rewrites files — RenameFile, sceneRename,
#     pathParser, defaultDataForPath. This library is hardlinked and seeding;
#     rename hooks are what the read-only mounts exist to prevent.
#   * ML stacks — visage, starIdentifier, AHavenVLMConnector, AIOverhaul pull
#     DeepFace and friends.
#   * Plugins needing credentials or services we do not run — TPDBMarkers
#     (ThePornDB key), PlexSync, timestampTrade.
#   * PythonDepManager / PythonToolsInstaller, whose whole job is pip-installing
#     at runtime into what is a read-only store symlink.
#
# The Python ones that remain work because stashapp-tools is packaged and put on
# the service's PATH — see modules/containers/stash.nix.
#
# Pinned by commit: upstream tags nothing and plugins track Stash's API, so a
# bump is a deliberate act with a diff to read.
{
  lib,
  stdenvNoCC,
  fetchFromGitHub,
}:
let
  # Shared UI helper library several of the JS plugins load.
  ui = [ "CommunityScriptsUILibrary" ];
  # Pure JS/UI — playback and browsing quality of life, no hooks, no mutation.
  cosmetic = [
    "ThumbPreviews"
    "VideoScrollWheel"
    "scenePageRememberStates"
    "externalLinksEnhanced"
    "markerDeleteButton"
    "StashRandomButton"
    "stats"
  ];
  # Metadata hooks. filenameParser is the one that matters for identification:
  # it derives studio/date/performers/title from the filename on scan, which is
  # the fallback when StashDB has no fingerprint match. videoChapterMarkers and
  # untagRedundantTags only ever touch metadata, never files.
  metadata = [
    "filenameParser"
    "videoChapterMarkers"
    "untagRedundantTags"
  ];
  selected = ui ++ cosmetic ++ metadata;
in
stdenvNoCC.mkDerivation {
  pname = "stash-community-scripts";
  version = "0-unstable-2026-09-07";

  src = fetchFromGitHub {
    owner = "stashapp";
    repo = "CommunityScripts";
    rev = "9fae7ecce6843e257bab7c0abf00bac7d7654354";
    hash = "sha256-fC1h5FGBqrV40odAS8M8etdgkIGx/OjrcFP0T4jAxN8=";
  };

  dontBuild = true;

  # The module symlinks each package's contents into plugins_path, so the store
  # output must BE the plugin tree. Fail loudly on a name that no longer exists
  # upstream rather than silently shipping fewer plugins after a bump.
  installPhase = ''
    runHook preInstall
    mkdir -p $out
    for p in ${lib.escapeShellArgs selected}; do
      if [ ! -d "plugins/$p" ]; then
        echo "plugin '$p' not found in CommunityScripts at this rev" >&2
        exit 1
      fi
      cp -r "plugins/$p" "$out/$p"
    done
    runHook postInstall
  '';

  meta = {
    description = "Curated community plugins for Stash";
    homepage = "https://github.com/stashapp/CommunityScripts";
    license = lib.licenses.agpl3Only;
    platforms = lib.platforms.all;
  };
}
