{ stdenvNoCC, python3, makeWrapper }:
stdenvNoCC.mkDerivation {
  name = "cloud-savegame";

  src = ./.;

  buildInputs = [ python3 ];

  nativeBuildInputs = [ makeWrapper ];

  dontUnpack = true;

  installPhase = ''
    mkdir $out/bin -p
    makeWrapper ${python3.interpreter} $out/bin/cloud-savegame \
      --add-flags $src/backup.py
  '';
}
