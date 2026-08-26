{ lib, buildGoModule }:

buildGoModule {
  pname = "cloud-savegame";
  version = builtins.replaceStrings ["\n"] [""] (builtins.readFile ./VERSION);

  src = ./.;

  vendorHash = "sha256-8pPHJz311PlTU9OkSVv6xUmdB1eYbcHmaOVvO1fNRyE=";

  subPackages = [ "cmd/cloud-savegame" ];

  postInstall = ''
    mv $out/bin/cloud-savegame $out/bin/cloud_savegame
  '';

  meta = with lib; {
    description = "Cloud Savegame Backup Tool";
    mainProgram = "cloud_savegame";
    license = licenses.mit;
  };
}
