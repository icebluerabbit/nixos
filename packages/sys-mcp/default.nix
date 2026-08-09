{ buildGoModule }:
buildGoModule {
  pname = "sys-mcp";
  version = "0.1.0";
  src = ./.;
  vendorHash = "sha256-1o1lTI/Zhx+1k1ISLVyG+7Jf5t2hVbr4hwCveW6IJlo=";
  meta.mainProgram = "sys-mcp";
}
