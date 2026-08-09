# Kodi/XBMC metadata consumer — writes album/artist NFO + artwork as SIDECAR files
# (cover.jpg, fanart, *.nfo) next to the tracks. Never touches the audio files, so it's
# safe for the hardlinked/seeding library. Artwork is fetched from Lidarr's metadata
# source (fanart.tv), independent of what the download peer shared.
_: {
  name = "Kodi";
  implementation = "XbmcMetadata";
  enable = true;
  artist_metadata = true;
  album_metadata = true;
  artist_images = true;
  album_images = true;
}
