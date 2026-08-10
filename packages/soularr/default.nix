/*
  soularr — bridges Lidarr and slskd: reads Lidarr's wanted list, searches Soulseek
  through slskd, downloads matches and imports them back.
  https://github.com/mrusse/soularr

  Ships two entrypoints — `soularr` (the batch run) and `soularr-webui` (the Flask
  config/monitor UI). Neither upstream nor its three Python dependencies (pyarr,
  slskd-api, music-tag) are in nixpkgs, so all four are built here.

  Pinned to a revision, not a tag: upstream tags lag the webui work that
  modules/containers/soularr is written against.
*/
{
  python3,
  fetchPypi,
  fetchFromGitHub,
  stdenvNoCC,
  makeWrapper,
}:
let
  py = python3.pkgs;

  music-tag = py.buildPythonPackage rec {
    pname = "music-tag";
    version = "0.4.3";
    pyproject = true;
    src = fetchPypi {
      inherit pname version;
      hash = "sha256-Cqtubu2o3w9TFuwtIZC9dFYbfgNWKrCRzo1Wh828//Y=";
    };
    build-system = [ py.setuptools ];
    dependencies = [ py.mutagen ];
    pythonImportsCheck = [ "music_tag" ];
  };

  slskd-api = py.buildPythonPackage rec {
    pname = "slskd-api";
    version = "0.1.5";
    pyproject = true;
    src = fetchPypi {
      inherit pname version;
      hash = "sha256-LmWP7bnK5IVid255qS2NGOmyKzGpUl3xsO5vi5uJI88=";
    };
    build-system = [
      py.setuptools
      py.setuptools-git-versioning
    ];
    dependencies = [ py.requests ];
    pythonImportsCheck = [ "slskd_api" ];
  };

  pyarr = py.buildPythonPackage rec {
    pname = "pyarr";
    version = "5.2.0";
    pyproject = true;
    src = fetchPypi {
      inherit pname version;
      hash = "sha256-jlcc9Kj1MYSsnvJkKZXXWWJVDx3KIuojjbGtl8kDUpw=";
    };
    # Upstream declares the legacy backend `poetry.masonry.api`, which
    # poetry-core does not provide (only `poetry.core.masonry.api`).
    postPatch = ''
      substituteInPlace pyproject.toml \
        --replace-fail 'poetry.masonry.api' 'poetry.core.masonry.api'
    '';
    build-system = [ py.poetry-core ];
    dependencies = [
      py.requests
      py.overrides
      py.types-requests
    ];
    pythonImportsCheck = [ "pyarr" ];
  };

  pythonEnv = python3.withPackages (ps: [
    pyarr
    slskd-api
    music-tag
    ps.flask
    ps.waitress
    ps.mutagen
  ]);
in
stdenvNoCC.mkDerivation {
  pname = "soularr";
  version = "0-unstable-2026-04-27";
  src = fetchFromGitHub {
    owner = "mrusse";
    repo = "soularr";
    rev = "f9e0ab922fd928a6d5d39cc9ddc0b0734006ddac";
    hash = "sha256-gtz99+DiFjJZuq54qo5C+5Exx++S+ePzldgDM9NHAOA=";
  };
  nativeBuildInputs = [ makeWrapper ];
  dontConfigure = true;
  dontBuild = true;
  installPhase = ''
    runHook preInstall
    mkdir -p $out/share/soularr
    cp -r soularr.py webui resources $out/share/soularr/
    makeWrapper ${pythonEnv}/bin/python $out/bin/soularr \
      --add-flags "$out/share/soularr/soularr.py"
    makeWrapper ${pythonEnv}/bin/python $out/bin/soularr-webui \
      --add-flags "$out/share/soularr/webui/webui.py"
    runHook postInstall
  '';
  meta = {
    description = "Lidarr-to-slskd bridge that fills the wanted list from Soulseek";
    homepage = "https://github.com/mrusse/soularr";
    mainProgram = "soularr";
  };
}
