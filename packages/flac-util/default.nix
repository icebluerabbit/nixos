{ buildGoModule }:
buildGoModule {
  pname = "flac-util";
  version = "0.1.0";
  src = ./.;
  vendorHash = "sha256-fAWGPjxNNvKqkfCWBKWHvN/UX2JZ17eCjss9Saxbq4Y=";
  meta.mainProgram = "flac-util";
}
