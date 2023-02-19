with import <nixpkgs> {};

buildGoModule {
  name = "wInd3x";
  src = ./.;

  buildInputs = [ libusb ];
  nativeBuildInputs = [ pkg-config ];

  vendorSha256 = "sha256-AjmH5oE/u+0R5d40r2zMAauQJTAwqHi24aqUMj2DmzU=";
}
