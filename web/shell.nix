with import <nixpkgs> {};

let
  tsgo = pkgs.buildGoModule {
    name = "tsgo-2025-03-05";
    src = pkgs.fetchFromGitHub {
      owner = "microsoft";
      repo = "typescript-go";
      rev = "5778a352bad26e285bec4da5f0e831140485522d";
      hash = "sha256-CfihPE6Wds0UqNfATCvAa4i0G9k85hy4bS6btSPrdjM=";
    };
    vendorHash = "sha256-HCoWmTdBDACcRHdzZA4Z4TYvoDBnt778w+Twhw+vId8=";
    doCheck = false;
  };

in pkgs.mkShell {
  buildInputs = with pkgs; [
    esbuild yarn tsgo
  ];
}
