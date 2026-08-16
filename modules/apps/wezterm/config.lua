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

-- Default is 16, which cuts task titles down to nothing. WezTerm still shrinks
-- tabs to share the window width, so this is a ceiling, not a reservation.
config.tab_max_width = 32
config.hide_tab_bar_if_only_one_tab = true
config.tab_bar_at_bottom = true
config.use_fancy_tab_bar = false
config.tab_and_split_indices_are_zero_based = false

local stylix_scheme = nil
pcall(function()
  stylix_scheme = wezterm.color.load_scheme(wezterm.config_dir .. "/colors/stylix.toml")
end)

-- Tab bar.
--
-- Rendered directly rather than through tabline.wez: the divider between tabs
-- needs a colour of its own, and tabline cannot express that. Its theme exposes
-- only tab.active / tab.inactive / tab.inactive_hover as { fg, bg }, and it
-- draws separators with fg taken from the tab's own bg
-- (tabs.lua: inactive_separator_attributes), so a divider is either invisible or
-- reads as a gap belonging to one of the two tabs. Every tabline_* section here
-- was empty anyway, so the plugin was only drawing tabs.
local ansi = (stylix_scheme and stylix_scheme.ansi) or {}
local tab_bar = (stylix_scheme and stylix_scheme.tab_bar) or {}
local scheme_bg = (stylix_scheme and stylix_scheme.background) or "#1e1e2e"

local BAR_BG = tab_bar.background or "#313244"
local TAB_ACTIVE = tab_bar.active_tab or { bg_color = scheme_bg, fg_color = "#cdd6f4" }
local TAB_INACTIVE = tab_bar.inactive_tab or { bg_color = "#6c7086", fg_color = "#cdd6f4" }
local TAB_HOVER = tab_bar.inactive_tab_hover or TAB_INACTIVE

-- Claude Code session state, published by modules/cli/claude/hooks/wezterm-tab.sh
-- into the tab title as "<state>:<epoch>". State shows purely as the background
-- colour of the tab while it is unfocused; nothing is added to the tab content.
local AGENT = {
    busy = ansi[4] or "#f9e2af",
    attention = ansi[2] or "#f38ba8",
    idle = ansi[5] or "#89b4fa",
    done = ansi[3] or "#a6e3a1",
}

-- Claude Code names each pane after the task it is working on, which is the only
-- thing that tells these tabs apart: the foreground process is ".claude-unwrapped"
-- in every one and every session runs out of the same checkout, so neither the
-- process name nor the cwd distinguishes them. Strip the status glyph Claude
-- prefixes the title with, and the -wrapped suffix Nix binaries carry.
local function tab_label(tab)
    local title = tostring(tab.active_pane.title or "")
    title = title:gsub("^[^%w]+", ""):gsub("%-u?n?wrapped$", "")
    if title == "" then
        title = tab.active_pane.domain_name or "wezterm"
    end
    return title
end

wezterm.on("format-tab-title", function(tab, _, _, _, hover, max_width)
    local fg, bg
    if tab.is_active then
        fg, bg = TAB_ACTIVE.fg_color, TAB_ACTIVE.bg_color
    else
        -- Tint unfocused tabs only; the focused one you are already looking at.
        local agent = AGENT[tostring(tab.tab_title):match("^(%a+):%d+$") or ""]
        local base = hover and TAB_HOVER or TAB_INACTIVE
        fg = agent and scheme_bg or base.fg_color
        bg = agent or base.bg_color
    end

    -- A blank cell in the terminal background colour, so the gap belongs to
    -- neither neighbour and stays the same whatever the two tabs are coloured.
    local items = {}
    local divider_width = 0
    if tab.tab_index > 0 then
        divider_width = 1
        table.insert(items, { Background = { Color = scheme_bg } })
        table.insert(items, { Text = " " })
    end

    -- Truncate here rather than letting WezTerm cut the tab off: it trims from
    -- the right, which would eat the trailing padding first.
    local prefix = " " .. tostring(tab.tab_index + 1) .. " "
    local budget = max_width - wezterm.column_width(prefix) - divider_width - 1
    local label = tab_label(tab)
    if budget < 1 then
        budget = 1
    end
    if wezterm.column_width(label) > budget then
        label = wezterm.truncate_right(label, budget - 1) .. "…"
    end

    table.insert(items, { Background = { Color = bg } })
    table.insert(items, { Foreground = { Color = fg } })
    table.insert(items, { Text = prefix .. label .. " " })
    return items
end)

-- The strip either side of the tabs, and the retro tab bar's own defaults for
-- anything drawn outside format-tab-title (the new-tab button).
config.colors = config.colors or {}
config.colors.tab_bar = {
  background = BAR_BG,
  active_tab = TAB_ACTIVE,
  inactive_tab = TAB_INACTIVE,
  inactive_tab_hover = TAB_HOVER,
  new_tab = tab_bar.new_tab or TAB_INACTIVE,
  new_tab_hover = tab_bar.new_tab_hover or TAB_HOVER,
}

-- Focusing a tab acknowledges its state, otherwise the tint would come back
-- every time you switched away from a tab you had already dealt with. "busy" is
-- exempt: a session left running in the background should stay marked as working.
wezterm.on("update-status", function(window)
    if window.is_focused and not window:is_focused() then
        return
    end
    local tab = window:active_tab()
    if not tab then
        return
    end
    local state = tostring(tab:get_title()):match("^(%a+):%d+$")
    if state and state ~= "busy" then
        tab:set_title("")
    end
end)

return config
