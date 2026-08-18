/*
  flac-util — measure FLAC files and record what was found beside each one.

  The collector shells out to two programs for measurements its own decoder
  cannot make: the reference FLAC decoder, for a second opinion on whether a
  file reads cleanly, and chromaprint, for an acoustic fingerprint of the
  recording. Both are wrapped onto PATH so a report never silently lacks them
  because of where it was run from; `--skip-external` omits those stages when
  the extra pass is not wanted.
*/
{
  buildGoModule,
  makeWrapper,
  lib,
  flac,
  chromaprint,
}:
buildGoModule {
  pname = "flac-util";
  version = "0.1.0";
  src = ./.;
  vendorHash = "sha256-8EzNNblY7V8kRyUuPVquqDqiDR98PfvJz6FA1MSayz0=";

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/flac-util \
      --prefix PATH : ${
        lib.makeBinPath [
          flac
          chromaprint
        ]
      }
  '';

  meta.mainProgram = "flac-util";
}
