# CommunityScrapers — the site-specific scraper collection every Stash install
# is expected to have. nixpkgs packages Stash itself but ships no scrapers, and
# services.stash.scrapers is a listOf package that defaults to [], so without
# this Stash can only identify what StashDB matches by fingerprint or filename.
#
# Pinned by commit: upstream tags nothing and scrapers break as sites change,
# so a bump is a deliberate act with a diff to read.
{
  lib,
  stdenvNoCC,
  fetchFromGitHub,
}:
stdenvNoCC.mkDerivation {
  pname = "stash-community-scrapers";
  version = "0-unstable-2026-09-07";

  src = fetchFromGitHub {
    owner = "stashapp";
    repo = "CommunityScrapers";
    rev = "d3044cf913c28277ed57af8d66e4a6aaffdf4405";
    hash = "sha256-LB09bbBmp5ibEUV4TpR4JdFAIkmFOzX2r9lOFtRQ7D4=";
  };

  dontBuild = true;

  # The module symlinks each package's contents into scrapers_path, so the
  # store output must BE the scraper tree, not a directory containing it.
  installPhase = ''
    runHook preInstall
    cp -r scrapers $out
    runHook postInstall
  '';

  meta = {
    description = "Community-maintained scrapers for Stash";
    homepage = "https://github.com/stashapp/CommunityScrapers";
    license = lib.licenses.agpl3Only;
    platforms = lib.platforms.all;
  };
}
