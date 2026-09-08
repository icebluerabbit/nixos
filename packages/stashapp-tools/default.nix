# stashapp-tools — the Python client for Stash's GraphQL API, imported as
# `stashapi`. Not in nixpkgs, and every Python plugin in CommunityScripts opens
# with `from stashapi.stashapp import StashInterface`, so without it that half
# of the plugin collection cannot load at all.
#
# The repo's own answer to this is PythonDepManager/PythonToolsInstaller, which
# pip-install at runtime — impossible here, since the plugin tree is a read-only
# symlink into the store. Packaging it is the declarative equivalent.
{
  lib,
  python3Packages,
  fetchPypi,
}:
python3Packages.buildPythonPackage rec {
  pname = "stashapp-tools";
  version = "0.2.59";
  pyproject = true;

  src = fetchPypi {
    inherit pname version;
    hash = "sha256-Y52YueWHp8C2FsnJ01YMBkz4O2z4d7RBeCswWGr8SjY=";
  };

  build-system = [ python3Packages.setuptools ];

  dependencies = [ python3Packages.requests ];

  # No test suite ships in the sdist; the import is the useful smoke check.
  doCheck = false;
  pythonImportsCheck = [ "stashapi" ];

  meta = {
    description = "Python library for interfacing with a stashapp's API";
    homepage = "https://github.com/stg-annon/stashapp-tools";
    license = lib.licenses.agpl3Only;
  };
}
