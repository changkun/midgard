# The disk image's window, for dmgbuild, which apple/build.sh runs: Midgard
# on the left, Applications on the right, and between them the arrow of
# background.svg (drawn at 1x and 2x in background.png and background@2x.png,
# which build.sh joins into one TIFF, the form Finder shows).
#
# The background is light: on a picture, Finder draws the labels under the
# icons black, whatever the Mac's appearance.

app = defines["app"]  # noqa: F821, as dmgbuild defines it
background = defines["background"]  # noqa: F821

format = "UDZO"
files = [app]
symlinks = {"Applications": "/Applications"}

window_rect = ((200, 140), (660, 400))
icon_size = 128
text_size = 13
# where background.svg leaves room for them, and its arrow goes between
icon_locations = {"Midgard.app": (165, 160), "Applications": (495, 160)}

default_view = "icon-view"
show_status_bar = False
show_tab_view = False
show_toolbar = False
show_pathbar = False
show_sidebar = False
show_icon_preview = False
include_icon_view_settings = True
