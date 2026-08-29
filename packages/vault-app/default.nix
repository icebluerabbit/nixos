/*
  vault-app — Vault, the local AI media tagger that runs against the LUKS vault
  https://github.com/aericocode/Vault

  Proprietary, source-available software: free for personal use on machines you
  own, no redistribution. That is why it is pinned by revision and built from
  source here rather than vendored or cached anywhere shared.

  Pinned to a commit, not a tag: upstream ships releases as Windows zips and
  tags lag the branch, so a rev is the only honest pin.
*/
{
  lib,
  buildNpmPackage,
  fetchFromGitHub,
  runCommand,
  srcOnly,
  nodejs,
  python3,
  node-gyp,
  ffmpeg-headless,
  chromaprint,
  zenity,
  makeWrapper,
}:
let
  # better-sqlite3-multiple-ciphers — the encrypted-database backend behind
  # Vault's own passphrase lock — bundles the SQLite3MultipleCiphers amalgamation
  # and includes it as <sqlite3.h>. Node 22 and 24 ship a plain sqlite3.h of their
  # own (for node:sqlite) in the very header directory node-gyp puts on the
  # include path first, so the addon compiles against a header that declares no
  # sqlite3_rekey and the build dies. Node 20, which predates that header, is no
  # longer in nixpkgs.
  #
  # So hand node-gyp the same header tree with those two files removed, and
  # <sqlite3.h> resolves to the bundled amalgamation as upstream intends. Symlinks
  # rather than copies — this is ~100 MB of Node sources to drop two files.
  nodeHeaders = runCommand "nodejs-headers-without-sqlite" { } ''
    cp -rs ${srcOnly nodejs} $out
    find $out -type d -exec chmod u+w {} +
    rm $out/include/node/sqlite3.h $out/include/node/sqlite3ext.h
  '';
in
buildNpmPackage {
  pname = "vault-app";
  version = "3.5.0-unstable-2026-08-16";

  src = fetchFromGitHub {
    owner = "aericocode";
    repo = "Vault";
    rev = "02cdc6b7f9169c224f3799790e74d69643f751b5";
    hash = "sha256-lvM4Fp12GEI5vHC4u2kdN5vKusDO4y3F2KyoTCy4b68=";
  };

  npmDepsHash = "sha256-C6hb8cMAz7/69hIQNH+5uGYXuTgu/q67d9/2dvziWIM=";

  # Upstream's "Add folder"/"Add files" buttons drive a PowerShell dialog and 501
  # everywhere else, telling the user to type a path into an input the viewer
  # does not have — so on Linux nothing can be imported from the UI at all.
  patches = [ ./picker-zenity.patch ];

  # The build-time-only deps (esbuild, postject) exist to produce the Windows
  # single-executable bundle, which this derivation never does.
  npmFlags = [ "--omit=dev" ];

  # better-sqlite3 and better-sqlite3-multiple-ciphers ship prebuilt binaries
  # that prebuild-install fetches over the network — unavailable in the sandbox,
  # so both fall back to node-gyp and compile here.
  nativeBuildInputs = [
    python3
    node-gyp
    makeWrapper
  ];

  # Upstream resolves its app root from the script location, which lands in the
  # store where the viewer's own settings writes (vault-settings.json) fail with
  # EROFS. VAULT_APP_ROOT moves that root to a writable directory — packages/vault
  # points it inside the mounted vault, next to the library it belongs to.
  postPatch = ''
    substituteInPlace lib/approot.js \
      --replace-fail \
        "const ROOT = isSea ? path.dirname(process.execPath) : path.join(__dirname, '..');" \
        "const ROOT = process.env.VAULT_APP_ROOT || (isSea ? path.dirname(process.execPath) : path.join(__dirname, '..'));"
  '';

  dontNpmBuild = true;

  # A flag, not npm_config_nodedir: npmConfigHook exports that variable itself
  # (and runs the rebuild) after postPatch, so anything exported earlier is
  # overwritten. npm's CLI config outranks its environment config, so this wins.
  npmRebuildFlags = [ "--nodedir=${nodeHeaders}" ];

  # ffmpeg/ffprobe/fpcalc are spawned by bare name (lib/ffmpeg-locate.js), and
  # upstream's fallback is an in-app downloader that drops Windows .exe files
  # next to itself. Put them on PATH so that path is never taken.
  postInstall = ''
    wrapProgram $out/bin/vault \
      --prefix PATH : ${
        lib.makeBinPath [
          ffmpeg-headless
          chromaprint
          zenity
        ]
      }
  '';

  # The viewer serves these from its app root, so they have to be reachable from
  # wherever VAULT_APP_ROOT points. packages/vault links this directory in.
  passthru.assets = [
    "css"
    "images"
    "player-lib"
    "db-viewer.html"
    "index.html"
    "gamify-config.json"
  ];

  meta = {
    description = "Local AI-powered media tagger and viewer";
    homepage = "https://github.com/aericocode/Vault";
    license = lib.licenses.unfree;
    mainProgram = "vault";
    platforms = lib.platforms.linux;
  };
}
