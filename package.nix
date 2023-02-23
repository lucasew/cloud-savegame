{ stdenvNoCC, python3 }:
stdenvNoCC.mkDerivation {
  name = "cloud-savegame";

  buildInputs = [ python3 ];

  dontUnpack = true;

  installPhase = ''
    mkdir $out/bin -p
    install ${./backup.py} $out/bin/cloud-savegame
    sed 's;\(DEFAULT_CONFIG_FILE =\)[^$]*;\1"${./demo.cfg}";'  -i $out/bin/cloud-savegame
  '';
}
