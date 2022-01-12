with import <nixpkgs> {};

buildGoModule {
  name = "wInd3x";
  src = ./.;

  buildInputs = [ libusb ];
  nativeBuildInputs = [ pkg-config ];

  vendorSha256 = "sha256-oislzCL1LylKpsDjaTb8rGiTi0qWD5k3vJr8SXg1rQg=";
}
