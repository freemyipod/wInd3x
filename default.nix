with import <nixpkgs> {};

buildGoModule {
  name = "wInd3x";
  src = ./.;

  buildInputs = [ libusb ];
  nativeBuildInputs = [ pkg-config ];

  vendorSha256 = "sha256-iFy7+TdTk4mVrtcW/smLkMxsdbh6Z2/aGpR+VTR0vCY=";
}
