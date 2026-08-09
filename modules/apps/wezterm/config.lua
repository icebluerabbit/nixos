local wezterm = require("wezterm")
local config = wezterm.config_builder()

-- leader active indicator prefix
local leader_prefix = utf8.char(0x1f30a) -- ocean wave

-- config.color_scheme = 'catppuccin-mocha'
config.enable_wayland = true

-- config.font = wezterm.font 'JetBrains Mono Nerd Font Mono'
config.font = wezterm.font("JetBrainsMono Nerd Font")
config.font_size = 14.0
config.harfbuzz_features = { "calt=1", "clig=1", "liga=1" }

config.window_padding = { left = 16, right = 16, top = 16, bottom = 16 }
config.window_background_opacity = 1.0
config.window_decorations = "NONE"

--config.front_end = "WebGpu"
--config.webgpu_power_preference = "HighPerformance"
config.max_fps = 120
config.animation_fps = 120

config.window_close_confirmation = "NeverPrompt"
config.audible_bell = "Disabled"
config.adjust_window_size_when_changing_font_size = false

config.enable_scroll_bar = false
config.automatically_reload_config = true
config.bold_brightens_ansi_colors = "BrightAndBold"
config.scrollback_lines = 10000

config.leader = {
    key = "a",
    mods = "CTRL",
    timeout_milliseconds = 2000,
};

config.quick_select_patterns = {
  [[(?m)https?://[^\s]+]],
}

config.keys = {
    {
        key = 'v',
        mods = 'CTRL|SHIFT',
        action = wezterm.action_callback(function(window, pane)
            local handle = io.popen('wl-paste --no-newline 2>/dev/null')
            if handle == nil then return end
            local clipboard = handle:read('*a')
            handle:close()
            window:perform_action(wezterm.action.SendString(clipboard), pane)
        end),
    },
    {
        mods = "LEADER",
        key = "c",
        action = wezterm.action.SpawnTab "CurrentPaneDomain"
    },
    {
        mods = "LEADER",
        key = "x",
        action = wezterm.action.CloseCurrentPane { confirm = true }
    },
}

for i = 1, 9 do
    table.insert(config.keys, {
        mods = "LEADER",
        key = tostring(i),
        action = wezterm.action.ActivateTab(i - 1)
    })
end

table.insert(config.keys, {
    mods = "LEADER",
    key = "0",
    action = wezterm.action.ActivateTab(9)
})

if not config.mouse_bindings then
    config.mouse_bindings = {}
end

table.insert(config.mouse_bindings, {
    event = { Up = { streak = 1, button = 'Left' } },
    mods = 'NONE',
    action = wezterm.action.CompleteSelection 'Clipboard',
})

config.hide_tab_bar_if_only_one_tab = true
config.tab_bar_at_bottom = true
config.use_fancy_tab_bar = false
config.tab_and_split_indices_are_zero_based = false

local stylix_scheme = nil
pcall(function()
  stylix_scheme = wezterm.color.load_scheme(wezterm.config_dir .. "/colors/stylix.toml")
end)

tabline.setup({
  options = {
    theme = stylix_scheme or 'Catppuccin Macchiato',
    tabs_enabled = true,
    tab_separators = {
      left = '',
      right = '',
    },
  },
  sections = {
    tabline_a = {},
    tabline_b = {},
    tabline_c = {},
    tabline_x = {},
    tabline_y = {},
    tabline_z = {},
  }
})

return config
