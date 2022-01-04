with import <nixpkgs> {};

buildGoModule {
  name = "wInd3x";
  src = ./.;

  buildInputs = [ libusb keystone ];
  nativeBuildInputs = [ pkg-config ];

  vendorSha256 = "sha256-M+hmCwQXr8KVyJH9CPNLkiOvkAPSWt8jTeRJaWKruIs=";
}
