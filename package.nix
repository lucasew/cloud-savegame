{ stdenvNoCC, python3, makeWrapper }:
stdenvNoCC.mkDerivation {
  name = "cloud-savegame";

  src = ./.;

  buildInputs = [ python3 ];

  nativeBuildInputs = [ makeWrapper ];

  dontUnpack = true;

  installPhase = ''
    mkdir $out/bin -p
    makeWrapper $src/backup.py $out/bin/cloud-savegame \
      --prefix ${python3.interpreter}
  '';
}
