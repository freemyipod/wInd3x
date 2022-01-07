with import <nixpkgs> {};

buildGoModule {
  name = "wInd3x";
  src = ./.;

  buildInputs = [ libusb ];
  nativeBuildInputs = [ pkg-config ];

  vendorSha256 = "sha256-iJmyb2HDJbdbu/Gd6yA5jecJSUSvjQVnfvw5m//306A=";
}
