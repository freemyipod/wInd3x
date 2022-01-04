with import <nixpkgs> {};

buildGoModule {
  name = "wInd3x";
  src = ./.;

  buildInputs = [ libusb keystone ];
  nativeBuildInputs = [ pkg-config ];

  vendorSha256 = "sha256-FXdpIO0UjIp5G7E5GOwsYJCHrTx7rA/cnrtn+pHVukU=";
}
