with import <nixpkgs> {};

buildGoModule {
  name = "wInd3x";
  src = ./.;

  buildInputs = [ libusb1 ];
  nativeBuildInputs = [ pkg-config ];

  deCheck = false;

  vendorHash = "sha256-QOgB7OD11L3Xj+ZOA4/UP88iEwVHXvSCMmdlkSwPu80=";
}
