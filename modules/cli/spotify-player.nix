/*
  spotify_player — terminal Spotify client with full playback control via the Spotify Connect API; supports librespot streaming
  https://github.com/aome510/spotify-player
*/
_: {
  den.aspects.cli._.spotify-player = {
    aspect-name = "cli.spotify-player";
    homeManager = {
      programs.spotify-player = {
        enable = true;

        settings = {
          # client_id = "";
          # client_id_command = { command = ""; args = []; };
          # login_redirect_uri = "http://127.0.0.1:8989/login";
          # client_port = 8080;
          # tracks_playback_limit = 50;
          # playback_format = "{status} {track} • {artists} {liked}\n{album} • {genres}\n{metadata}";
          # playback_metadata_fields = ["repeat" "shuffle" "volume" "device"];
          # notify_format = { summary = "{track} • {artists}"; body = "{album}"; };
          # notify_timeout_in_secs = 0;
          # notify_transient = false;
          # theme = "default";
          # app_refresh_duration_in_ms = 32;
          # playback_refresh_duration_in_ms = 0;
          # page_size_in_rows = 20;
          # enable_media_control = true;
          # enable_streaming = "Always"; # Always | Never | DaemonOnly
          enable_audio_visualization = true;
          # enable_notify = true;
          # enable_cover_image_cache = true;
          # notify_streaming_only = false;
          # default_device = "spotify-player";
          # play_icon = "▶";
          # pause_icon = "▌▌";
          # liked_icon = "♥";
          # explicit_icon = "(E)";
          # border_type = "Plain";           # Hidden | Plain | Rounded | Double | Thick
          # progress_bar_type = "Rectangle"; # Rectangle | Line
          # progress_bar_position = "Bottom"; # Bottom | Right
          # enable_audio_visualization = false;
          # genre_num = 2;
          # seek_duration_secs = 5;
          # volume_scroll_step = 5;
          # enable_mouse_scroll_volume = true;
          # custom_queue = true;
          # sort_artist_albums_by_type = false;
          cover_img_length = 9;
          cover_img_width = 5;
          cover_img_scale = 3.5;
          # cover_img_pixels = 16; # requires pixelate feature
          # log_folder = null;
          # ap_port = null;
          # proxy = null;
          # player_event_hook_command = { command = ""; args = []; };

          device = {
            name = "spotify-player";
            # device_type = "speaker";
            volume = 100;
            bitrate = 320;
            audio_cache = true;
            normalization = false;
            autoplay = false;
          };

          layout = {
            playback_window_position = "Top"; # Top | Bottom
            playback_window_height = 6;
            # library = { playlist_percent = 40; album_percent = 40; };
          };
        };

        keymaps = [
          # Playback
          # { command = "NextTrack";                       key_sequence = "n"; }
          # { command = "PreviousTrack";                   key_sequence = "p"; }
          # { command = "ResumePause";                     key_sequence = "space"; }
          # { command = "PlayRandom";                      key_sequence = "."; }
          # { command = "Repeat";                          key_sequence = "C-r"; }
          # { command = "Shuffle";                         key_sequence = "C-s"; }
          # { command = { VolumeChange = { offset = 5; };  } key_sequence = "+"; }
          # { command = { VolumeChange = { offset = -5; }; } key_sequence = "-"; }
          # { command = "Mute";                            key_sequence = "_"; }
          # { command = "SeekStart";                       key_sequence = "^"; }
          # { command = { SeekForward  = {}; };            key_sequence = ">"; }
          # { command = { SeekBackward = {}; };            key_sequence = "<"; }

          # Navigation
          # { command = "SelectNextOrScrollDown";          key_sequence = "j"; }
          # { command = "SelectNextOrScrollDown";          key_sequence = "C-n"; }
          # { command = "SelectNextOrScrollDown";          key_sequence = "down"; }
          # { command = "SelectPreviousOrScrollUp";        key_sequence = "k"; }
          # { command = "SelectPreviousOrScrollUp";        key_sequence = "C-p"; }
          # { command = "SelectPreviousOrScrollUp";        key_sequence = "up"; }
          # { command = "PageSelectNextOrScrollDown";      key_sequence = "C-f"; }
          # { command = "PageSelectNextOrScrollDown";      key_sequence = "page_down"; }
          # { command = "PageSelectPreviousOrScrollUp";    key_sequence = "C-b"; }
          # { command = "PageSelectPreviousOrScrollUp";    key_sequence = "page_up"; }
          # { command = "SelectFirstOrScrollToTop";        key_sequence = "g g"; }
          # { command = "SelectFirstOrScrollToTop";        key_sequence = "home"; }
          # { command = "SelectLastOrScrollToBottom";      key_sequence = "G"; }
          # { command = "SelectLastOrScrollToBottom";      key_sequence = "end"; }
          # { command = "ChooseSelected";                  key_sequence = "enter"; }
          # { command = "FocusNextWindow";                 key_sequence = "tab"; }
          # { command = "FocusPreviousWindow";             key_sequence = "backtab"; }

          # Pages
          # { command = "CurrentlyPlayingContextPage";     key_sequence = "g space"; }
          # { command = "TopTrackPage";                    key_sequence = "g t"; }
          # { command = "RecentlyPlayedTrackPage";         key_sequence = "g r"; }
          # { command = "LikedTrackPage";                  key_sequence = "g y"; }
          # { command = "LyricsPage";                      key_sequence = "g L"; }
          # { command = "LyricsPage";                      key_sequence = "l"; }
          # { command = "LibraryPage";                     key_sequence = "g l"; }
          # { command = "SearchPage";                      key_sequence = "g s"; }
          # { command = "BrowsePage";                      key_sequence = "g b"; }
          # { command = "Queue";                           key_sequence = "z"; }
          # { command = "OpenCommandHelp";                 key_sequence = "?"; }
          # { command = "OpenCommandHelp";                 key_sequence = "C-h"; }
          # { command = "PreviousPage";                    key_sequence = "backspace"; }
          # { command = "PreviousPage";                    key_sequence = "C-q"; }
          # { command = "OpenLogs";                        key_sequence = "g o"; }

          # Actions/Popups
          # { command = "ShowActionsOnSelectedItem";       key_sequence = "g a"; }
          # { command = "ShowActionsOnSelectedItem";       key_sequence = "C-space"; }
          # { command = "ShowActionsOnCurrentTrack";       key_sequence = "a"; }
          # { command = "ShowActionsOnCurrentContext";     key_sequence = "A"; }
          # { command = "AddSelectedItemToQueue";          key_sequence = "Z"; }
          # { command = "AddSelectedItemToQueue";          key_sequence = "C-z"; }
          # { command = "Search";                          key_sequence = "/"; }
          # { command = "SwitchTheme";                     key_sequence = "T"; }
          # { command = "SwitchDevice";                    key_sequence = "D"; }
          # { command = "BrowseUserPlaylists";             key_sequence = "u p"; }
          # { command = "BrowseUserFollowedArtists";       key_sequence = "u a"; }
          # { command = "BrowseUserSavedAlbums";           key_sequence = "u A"; }
          # { command = "ClosePopup";                      key_sequence = "esc"; }

          # Sort
          # { command = "SortTrackByTitle";                key_sequence = "s t"; }
          # { command = "SortTrackByArtists";              key_sequence = "s a"; }
          # { command = "SortTrackByAlbum";                key_sequence = "s A"; }
          # { command = "SortTrackByAddedDate";            key_sequence = "s D"; }
          # { command = "SortTrackByDuration";             key_sequence = "s d"; }
          # { command = "SortLibraryAlphabetically";       key_sequence = "s l a"; }
          # { command = "SortLibraryByRecent";             key_sequence = "s l r"; }
          # { command = "ReverseOrder";                    key_sequence = "s r"; }

          # Misc
          # { command = "RefreshPlayback";                 key_sequence = "r"; }
          # { command = "RestartIntegratedClient";         key_sequence = "R"; }
          # { command = "MovePlaylistItemUp";              key_sequence = "C-k"; }
          # { command = "MovePlaylistItemDown";            key_sequence = "C-j"; }
          # { command = "CreatePlaylist";                  key_sequence = "N"; }
          # { command = "JumpToCurrentTrackInContext";     key_sequence = "g c"; }
          # { command = "JumpToHighlightTrackInContext";   key_sequence = "C-g"; }
          # { command = "OpenSpotifyLinkFromClipboard";    key_sequence = "O"; }
          # { command = "Quit";                            key_sequence = "C-c"; }
          # { command = "Quit";                            key_sequence = "q"; }
        ];

        actions = [
          # { action = "GoToArtist";          key_sequence = ""; }
          # { action = "GoToAlbum";           key_sequence = ""; }
          # { action = "GoToRadio";           key_sequence = ""; }
          # { action = "AddToLibrary";        key_sequence = ""; }
          # { action = "AddToPlaylist";       key_sequence = ""; }
          # { action = "AddToQueue";          key_sequence = ""; }
          # { action = "AddToLiked";          key_sequence = ""; }
          # { action = "DeleteFromLiked";     key_sequence = ""; }
          # { action = "DeleteFromLibrary";   key_sequence = ""; }
          # { action = "DeleteFromPlaylist";  key_sequence = ""; }
          # { action = "ToggleLiked";         key_sequence = ""; }
          # { action = "CopyLink";            key_sequence = ""; }
          # { action = "Follow";              key_sequence = ""; }
          # { action = "Unfollow";            key_sequence = ""; }
        ];
      };

      programs.zsh.shellAliases = {
        spotify = "spotify_player";
      };
    };
  };
}
