"""Pill-shaped kitty tabs.

Each tab is drawn as a rounded capsule: the two cap glyphs are painted in
the tab's own background colour as their foreground, over the tab bar's
background, so the colour reads as a continuous rounded block rather than
a rectangle with decorations on the ends.

This is the only way to get both caps. The built-in powerline style draws
a separator on the right of each tab and nothing on the left, and no
other built-in style rounds anything.

Nothing here names a colour. The cap follows whatever background the tab
already has, which is what lets the Claude Code hook's per-tab
set-tab-color overrides colour the whole pill without this file knowing
the states exist.

Loaded by kitty from its config directory under the fixed name tab_bar.py
-- kitty hardcodes the path, so unlike the watcher it cannot be pointed
at the nix store.
"""

from kitty.tab_bar import as_rgb, draw_title
from kitty.utils import color_as_int

LEFT_CAP = ""
RIGHT_CAP = ""

# Two caps plus the space either side of the title.
CHROME = 4


def draw_tab(
    draw_data,
    screen,
    tab,
    before,
    max_title_length,
    index,
    is_last,
    extra_data,
):
    tab_bg = screen.cursor.bg
    tab_fg = screen.cursor.fg
    default_bg = as_rgb(color_as_int(draw_data.default_bg))

    def cap(glyph):
        screen.cursor.bg = default_bg
        screen.cursor.fg = tab_bg
        screen.draw(glyph)
        screen.cursor.bg = tab_bg
        screen.cursor.fg = tab_fg

    if max_title_length <= CHROME:
        screen.draw("…")
        return screen.cursor.x

    cap(LEFT_CAP)
    screen.draw(" ")
    draw_title(draw_data, screen, tab, index, max_title_length - CHROME)

    # draw_title honours the template's own width handling but not this
    # tab's share of the bar, so clip whatever overran.
    extra = screen.cursor.x - before - (max_title_length - 2)
    if extra > 0:
        screen.cursor.x -= extra + 1
        screen.draw("…")

    screen.draw(" ")
    cap(RIGHT_CAP)

    # A gap between pills, in the bar background rather than either tab's.
    screen.cursor.bg = default_bg
    screen.cursor.fg = default_bg
    if not is_last:
        screen.draw(" ")
    return screen.cursor.x
