/*
  opencode-plugin-otel — OpenTelemetry plugin for opencode: exports agent sessions,
  tool calls and token usage over OTLP.
  https://github.com/DEVtheOPS/opencode-plugin-otel

  Consumed as a file path, not a binary — opencode's `plugin` setting takes a list
  of JS entrypoints, so the useful output is ${pkgs.homelab.opencode-plugin-otel}/dist/index.js.

  Fixed-output: `bun install` needs the network, so the whole build is hashed
  instead of a vendored lockfile. Bumping the rev means bumping outputHash too.
*/
{
  lib,
  stdenv,
  fetchFromGitHub,
  bun,
}:
stdenv.mkDerivation {
  pname = "opencode-plugin-otel";
  version = "1.2.1";

  src = fetchFromGitHub {
    owner = "DEVtheOPS";
    repo = "opencode-plugin-otel";
    rev = "09b039d0d42034f375b1aa33b4e2c81cc9204de6";
    hash = "sha256-jZ/h7gmQAIHozSRFucnSUoDQCzSaed1EMtRAe1KxReI=";
  };

  nativeBuildInputs = [ bun ];

  impureEnvVars = lib.fetchers.proxyImpureEnvVars;

  buildPhase = ''
    runHook preBuild
    export HOME=$(mktemp -d)
    export BUN_INSTALL_CACHE_DIR=$(mktemp -d)
    bun install --frozen-lockfile --ignore-scripts --no-progress
    bun build src/index.ts --outdir=./dist --target=node
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    mkdir -p $out/dist
    cp dist/index.js $out/dist/
    runHook postInstall
  '';

  outputHashAlgo = "sha256";
  outputHashMode = "recursive";
  outputHash = "sha256-UJPDm8FpwxV28VtiNj1PZ/nKezLTEelyeeJymUZO9mU=";

  meta = {
    description = "OpenTelemetry plugin for the opencode agent";
    homepage = "https://github.com/DEVtheOPS/opencode-plugin-otel";
  };
}
