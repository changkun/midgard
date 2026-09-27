#!/usr/bin/env python3
# Copyright 2026 Changkun Ou. All rights reserved.
# Use of this source code is governed by a GPL-3.0
# license that can be found in the LICENSE file.

"""Draws the hero, api/rest/web/images/hero.svg, from the screenshots here.

The Mac's menu bar, with Midgard's window open, and an iPhone with the web
page on its Home Screen. A flight's details are selected in a note and
copied; the copy shows at the top of Midgard's window, travels sealed past
the server, and opens on the phone. It loops every eight seconds, in CSS,
and holds still at its end for a viewer who asks for less motion.

One file, its pictures inside it, so that it works as an <img> on the web
page and in the README alike, where an SVG may load nothing else.

    python3 docs/images/hero.py

The screenshots come from the app's ScreenshotTests (mac-*) and from the web
page with a demo server's history (phone-*).
"""

import base64
import os
import random

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
OUT = os.path.join(ROOT, "api", "rest", "web", "images", "hero.svg")

W, H = 1600, 900
LOOP = 8  # seconds


def data(name):
    path = os.path.join(HERE, name)
    kind = "image/png" if name.endswith(".png") else "image/jpeg"
    with open(path, "rb") as f:
        return f"data:{kind};base64,{base64.b64encode(f.read()).decode()}"


# The Mac's screen: a wallpaper as the one the app was photographed over.
DX, DY, DW, DH = 56, 64, 1024, 772
BAR = 30
# Midgard's window, under its icon in the menu bar: a 360 x 480 panel,
# 23 and 19 points into its picture, which holds its shadow.
ICON_X = 868
PX, PY = ICON_X - 180, DY + BAR + 6
# The phone: the page at 0.8, under a status bar, in a bezel.
SP = 0.8
SW, SH = 393 * SP, 852 * SP
PHX, PHY = 1180, 124
BEZEL = 12
SX, SY = PHX + BEZEL, PHY + BEZEL
STATUS = 47 * SP
# The note the flight is copied from, and the keys that copy it, under it.
NX, NY, NW, NH = DX + 64, DY + 290, 540, 176
KX, KY = NX + NW / 2 - 64, NY + NH + 30

# The copy's way: out of Midgard's newest row, over the server, into the
# phone's clipboard.
START = (PX + 354, PY + 150)
SERVER = (1112, 58)
END = (SX + 150, SY + STATUS + 134)
PATH = (f"M {START[0]} {START[1]} C {START[0] + 10} {SERVER[1] - 20}, "
        f"{SERVER[0] - 40} {SERVER[1] - 26}, {SERVER[0]} {SERVER[1]} "
        f"S {END[0] - 40} {END[1] - 120}, {END[0]} {END[1]}")

FONT = "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Segoe UI', Inter, Helvetica, Arial, sans-serif"


def stars():
    r = random.Random(7)
    out = []
    for i in range(70):
        x, y = r.uniform(10, W - 10), r.uniform(10, H * 0.62)
        size = r.choice([0.8, 1, 1, 1.2, 1.6])
        twinkle = f' class="tw" style="animation-delay:-{r.uniform(0, 4):.1f}s"' if i % 4 == 0 else ""
        out.append(f'<circle cx="{x:.0f}" cy="{y:.0f}" r="{size}" fill="#fff" opacity="{r.uniform(.25, .8):.2f}"{twinkle}/>')
    return "\n    ".join(out)


def svg():
    popover = data("mac-popover-dark.png")
    popover_copied = data("mac-popover-copied-dark.png")
    phone = data("phone-dark.jpg")
    phone_copied = data("phone-copied-dark.jpg")
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" aria-label="A flight's details, copied on a Mac, show up in Midgard's menu bar window, travel encrypted past the server, and arrive on an iPhone">
  <title>Midgard: copy on your Mac, paste on your iPhone</title>
  <style>
    text {{ font-family: {FONT}; }}
    .tw {{ animation: tw 4s ease-in-out infinite alternate; }}
    @keyframes tw {{ from {{ opacity: .15 }} to {{ opacity: .9 }} }}
    .sel {{ transform-box: fill-box; transform-origin: left center; animation: sel {LOOP}s infinite; }}
    @keyframes sel {{ 0%, 4% {{ opacity: 0; transform: scaleX(0) }} 5% {{ opacity: .9 }} 10% {{ transform: scaleX(1) }}
      88% {{ opacity: .9; transform: scaleX(1) }} 92%, 100% {{ opacity: 0; transform: scaleX(1) }} }}
    .keys {{ animation: keys {LOOP}s infinite; }}
    @keyframes keys {{ 0%, 9% {{ opacity: 0; transform: translateY(10px) }} 11% {{ opacity: 1; transform: none }}
      14% {{ transform: translateY(3px) }} 16% {{ transform: none }} 22% {{ opacity: 1 }} 26%, 100% {{ opacity: 0 }} }}
    .copied {{ animation: copied {LOOP}s infinite; }}
    @keyframes copied {{ 0%, 16% {{ opacity: 0 }} 19%, 88% {{ opacity: 1 }} 92%, 100% {{ opacity: 0 }} }}
    .arrived {{ animation: arrived {LOOP}s infinite; }}
    @keyframes arrived {{ 0%, 44% {{ opacity: 0 }} 47%, 88% {{ opacity: 1 }} 92%, 100% {{ opacity: 0 }} }}
    .ring {{ animation: ring {LOOP}s infinite; }}
    @keyframes ring {{ 0%, 45% {{ opacity: 0 }} 48% {{ opacity: 1 }} 62%, 100% {{ opacity: 0 }} }}
    .track {{ animation: track {LOOP}s infinite; }}
    @keyframes track {{ 0%, 19% {{ opacity: 0 }} 22%, 43% {{ opacity: 1 }} 50%, 100% {{ opacity: 0 }} }}
    .packet {{ animation: packet {LOOP}s infinite; }}
    @keyframes packet {{ 0%, 19% {{ opacity: 0 }} 21%, 42% {{ opacity: 1 }} 45%, 100% {{ opacity: 0 }} }}
    .server {{ transform-box: fill-box; transform-origin: center; animation: server {LOOP}s infinite; }}
    @keyframes server {{ 0%, 29% {{ transform: none }} 32% {{ transform: scale(1.12) }} 36%, 100% {{ transform: none }} }}
    @media (prefers-reduced-motion: reduce) {{
      .tw, .sel, .keys, .copied, .arrived, .ring, .track, .packet, .server {{ animation: none; }}
      .copied, .arrived {{ opacity: 1; }}
      .keys, .ring, .track, .packet {{ opacity: 0; }}
    }}
  </style>
  <defs>
    <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#050a1c"/><stop offset=".55" stop-color="#0f1c52"/><stop offset="1" stop-color="#1b3a9a"/>
    </linearGradient>
    <radialGradient id="glow" cx=".55" cy=".2" r=".6">
      <stop offset="0" stop-color="#3c5bd8" stop-opacity=".45"/><stop offset="1" stop-color="#3c5bd8" stop-opacity="0"/>
    </radialGradient>
    <linearGradient id="wallpaper" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#0d1233"/><stop offset="1" stop-color="#2e215c"/>
    </linearGradient>
    <linearGradient id="trail" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0" stop-color="#7aa2ff" stop-opacity=".1"/><stop offset=".5" stop-color="#9db8ff"/><stop offset="1" stop-color="#7aa2ff" stop-opacity=".1"/>
    </linearGradient>
    <clipPath id="frame"><rect width="{W}" height="{H}" rx="28"/></clipPath>
    <clipPath id="screen"><rect x="{DX}" y="{DY}" width="{DW}" height="{DH}" rx="18"/></clipPath>
    <clipPath id="phone"><rect x="{SX}" y="{SY}" width="{SW}" height="{SH}" rx="46"/></clipPath>
    <filter id="lift" x="-10%" y="-10%" width="120%" height="130%">
      <feDropShadow dx="0" dy="24" stdDeviation="28" flood-color="#000" flood-opacity=".55"/>
    </filter>
    <filter id="soft" x="-20%" y="-20%" width="140%" height="160%">
      <feDropShadow dx="0" dy="10" stdDeviation="14" flood-color="#000" flood-opacity=".45"/>
    </filter>
    <filter id="bloom" x="-100%" y="-100%" width="300%" height="300%"><feGaussianBlur stdDeviation="6"/></filter>
  </defs>

  <g clip-path="url(#frame)">
  <rect width="{W}" height="{H}" fill="url(#sky)"/>
  <rect width="{W}" height="{H}" fill="url(#glow)"/>
  <g>
    {stars()}
  </g>
  <!-- the logo's mountains, along the bottom -->
  <path d="M0 900 L0 770 L180 700 L330 760 L520 650 L720 760 L900 690 L1080 770 L1260 660 L1440 740 L1600 690 L1600 900 Z" fill="#132a78" opacity=".8"/>
  <path d="M0 900 L0 820 L220 760 L400 820 L640 740 L860 830 L1060 770 L1300 840 L1480 790 L1600 820 L1600 900 Z" fill="#0a143d"/>

  <!-- the Mac -->
  <g filter="url(#lift)">
    <rect x="{DX}" y="{DY}" width="{DW}" height="{DH}" rx="18" fill="url(#wallpaper)"/>
  </g>
  <g clip-path="url(#screen)">
    <rect x="{DX}" y="{DY}" width="{DW}" height="{DH}" fill="url(#wallpaper)"/>
    <circle cx="{DX + 820}" cy="{DY + 640}" r="340" fill="#6a4fd6" opacity=".14"/>
    <circle cx="{DX + 140}" cy="{DY + 160}" r="260" fill="#2f5fe0" opacity=".12"/>

    <!-- the note the flight is copied from -->
    <g filter="url(#soft)">
      <rect x="{NX}" y="{NY}" width="{NW}" height="{NH}" rx="14" fill="#1a1f42" stroke="#ffffff" stroke-opacity=".08"/>
    </g>
    <rect x="{NX + 24}" y="{NY + 22}" width="24" height="24" rx="6" fill="#f5c542"/>
    <rect x="{NX + 29}" y="{NY + 30}" width="14" height="2.2" rx="1.1" fill="#7a5a00"/>
    <rect x="{NX + 29}" y="{NY + 35.5}" width="9" height="2.2" rx="1.1" fill="#7a5a00"/>
    <text x="{NX + 60}" y="{NY + 41}" font-size="18" font-weight="650" fill="#f2f4ff">Trip to Berlin</text>
    <text x="{NX + NW - 24}" y="{NY + 41}" font-size="14" fill="#9aa3c7" text-anchor="end">Notes</text>
    <rect class="sel" x="{NX + 20}" y="{NY + 76}" width="400" height="31" rx="5" fill="#3b6df0"/>
    <rect class="sel" x="{NX + 20}" y="{NY + 112}" width="248" height="31" rx="5" fill="#3b6df0" style="animation-delay:.12s"/>
    <text x="{NX + 25}" y="{NY + 99}" font-size="20" fill="#f2f4ff">Flight LH 2023 · Munich → Berlin ·</text>
    <text x="{NX + 25}" y="{NY + 135}" font-size="20" fill="#f2f4ff">Gate G24, boarding 18:05</text>
    <g class="keys">
      <rect x="{KX}" y="{KY}" width="60" height="60" rx="13" fill="#232a52" stroke="#ffffff" stroke-opacity=".22"/>
      <text x="{KX + 30}" y="{KY + 40}" font-size="27" fill="#fff" text-anchor="middle">⌘</text>
      <rect x="{KX + 68}" y="{KY}" width="60" height="60" rx="13" fill="#232a52" stroke="#ffffff" stroke-opacity=".22"/>
      <text x="{KX + 98}" y="{KY + 40}" font-size="25" font-weight="600" fill="#fff" text-anchor="middle">C</text>
    </g>

    <!-- the menu bar, with Midgard's icon, its window open -->
    <rect x="{DX}" y="{DY}" width="{DW}" height="{BAR}" fill="#0a0d24" opacity=".62"/>
    <text x="{DX + 24}" y="{DY + 20}" font-size="14" font-weight="700" fill="#eef0ff">Finder</text>
    <text x="{DX + 84}" y="{DY + 20}" font-size="14" fill="#eef0ff" opacity=".85" word-spacing="14">File Edit View Go Window Help</text>
    <rect x="{ICON_X - 16}" y="{DY + 3}" width="32" height="24" rx="6" fill="#ffffff" opacity=".18"/>
    <g transform="translate({ICON_X - 9} {DY + 6})" fill="#fff">
      <circle cx="9" cy="9" r="7.4" fill="none" stroke="#fff" stroke-width="1.6"/>
      <clipPath id="world"><circle cx="9" cy="9" r="6.6"/></clipPath>
      <g clip-path="url(#world)">
        <path d="M 2 17 L 6.1 8.4 L 10.2 17 Z" opacity=".65"/><path d="M 8.4 17 L 12.6 8 L 16.8 17 Z" opacity=".65"/>
        <path d="M 4.6 17 L 9.1 5.2 L 13.6 17 Z"/>
      </g>
    </g>
    <g transform="translate({DX + DW - 170} {DY + 9})" fill="#eef0ff" opacity=".9">
      <path d="M8 11.5 a1.3 1.3 0 1 1 0.01 0Z M3.2 7.6 a6.8 6.8 0 0 1 9.6 0 l-1.3 1.3 a5 5 0 0 0 -7 0Z M0.6 5 a10.5 10.5 0 0 1 14.8 0 l-1.3 1.3 a8.7 8.7 0 0 0 -12.2 0Z"/>
      <rect x="30" y="1" width="23" height="11" rx="3" fill="none" stroke="#eef0ff" stroke-width="1.2"/>
      <rect x="32" y="3" width="16" height="7" rx="1.5"/><rect x="54" y="4.5" width="1.8" height="4" rx=".9"/>
    </g>
    <text x="{DX + DW - 18}" y="{DY + 20}" font-size="14" fill="#eef0ff" text-anchor="end">Fri 17:42</text>

    <image href="{popover}" x="{PX - 23}" y="{PY - 19}" width="406" height="526"/>
    <image class="copied" href="{popover_copied}" x="{PX - 23}" y="{PY - 19}" width="406" height="526"/>
  </g>
  <rect x="{DX}" y="{DY}" width="{DW}" height="{DH}" rx="18" fill="none" stroke="#ffffff" stroke-opacity=".1"/>

  <!-- the iPhone, with the web page on its Home Screen -->
  <g filter="url(#lift)">
    <rect x="{PHX}" y="{PHY}" width="{SW + 2 * BEZEL}" height="{SH + 2 * BEZEL}" rx="58" fill="#0b0d14"/>
  </g>
  <rect x="{PHX - 3}" y="{PHY + 150}" width="3" height="56" rx="1.5" fill="#1d2130"/>
  <rect x="{PHX - 3}" y="{PHY + 220}" width="3" height="56" rx="1.5" fill="#1d2130"/>
  <rect x="{PHX + SW + 2 * BEZEL}" y="{PHY + 190}" width="3" height="86" rx="1.5" fill="#1d2130"/>
  <rect x="{PHX + 1.5}" y="{PHY + 1.5}" width="{SW + 2 * BEZEL - 3}" height="{SH + 2 * BEZEL - 3}" rx="56.5" fill="none" stroke="#3a3f52" stroke-width="2"/>
  <g clip-path="url(#phone)">
    <rect x="{SX}" y="{SY}" width="{SW}" height="{SH}" fill="#08132b"/>
    <image href="{phone}" x="{SX}" y="{SY + STATUS}" width="{SW}" height="{SH}"/>
    <image class="arrived" href="{phone_copied}" x="{SX}" y="{SY + STATUS}" width="{SW}" height="{SH}"/>
    <rect x="{SX}" y="{SY}" width="{SW}" height="{STATUS}" fill="#08132b"/>
    <text x="{SX + 40}" y="{SY + 26}" font-size="14" font-weight="650" fill="#fff" text-anchor="middle">17:42</text>
    <g transform="translate({SX + SW - 76} {SY + 16})" fill="#fff">
      <rect x="0" y="7" width="3" height="4" rx="1"/><rect x="5" y="5" width="3" height="6" rx="1"/>
      <rect x="10" y="3" width="3" height="8" rx="1"/><rect x="15" y="1" width="3" height="10" rx="1"/>
      <rect x="26" y="0.5" width="22" height="11" rx="3" fill="none" stroke="#fff" stroke-opacity=".5"/>
      <rect x="28" y="2.5" width="15" height="7" rx="1.5"/>
    </g>
    <rect x="{SX + SW / 2 - 50}" y="{SY + 9}" width="100" height="28" rx="14" fill="#000"/>
    <rect class="ring" x="{SX + 20}" y="{SY + STATUS + 94}" width="{SW - 40}" height="80" rx="14" fill="none" stroke="#8fb0ff" stroke-width="3"/>
    <rect x="{SX + SW / 2 - 54}" y="{SY + SH - 12}" width="108" height="4" rx="2" fill="#fff" opacity=".6"/>
  </g>

  <!-- the copy's way: sealed on the Mac, past the server, opened on the phone -->
  <path class="track" d="{PATH}" fill="none" stroke="url(#trail)" stroke-width="2.5" stroke-dasharray="2 9" stroke-linecap="round"/>
  <g class="server">
    <rect x="{SERVER[0] - 34}" y="{SERVER[1] - 22}" width="68" height="44" rx="12" fill="#18225a" stroke="#9db8ff" stroke-opacity=".45"/>
    <rect x="{SERVER[0] - 18}" y="{SERVER[1] - 11}" width="36" height="9" rx="3" fill="none" stroke="#c9d6ff" stroke-width="1.6"/>
    <rect x="{SERVER[0] - 18}" y="{SERVER[1] + 2}" width="36" height="9" rx="3" fill="none" stroke="#c9d6ff" stroke-width="1.6"/>
    <circle cx="{SERVER[0] + 11}" cy="{SERVER[1] - 6.5}" r="1.6" fill="#6ee7a8"/>
    <circle cx="{SERVER[0] + 11}" cy="{SERVER[1] + 6.5}" r="1.6" fill="#6ee7a8"/>
  </g>
  <text x="{SERVER[0]}" y="{SERVER[1] + 40}" font-size="13" fill="#c9d6ff" text-anchor="middle">your server</text>
  <text x="{SERVER[0]}" y="{SERVER[1] + 56}" font-size="13" fill="#8f9bc9" text-anchor="middle">can’t read it</text>
  <g class="packet">
    <g>
      <animateMotion dur="{LOOP}s" repeatCount="indefinite" calcMode="linear" keyPoints="0;0;1;1" keyTimes="0;0.2;0.44;1" path="{PATH}"/>
      <circle r="16" fill="#7aa2ff" opacity=".7" filter="url(#bloom)"/>
      <rect x="-15" y="-15" width="30" height="30" rx="9" fill="#2f5fe0" stroke="#c9d6ff" stroke-opacity=".7"/>
      <rect x="-6" y="-2" width="12" height="9" rx="2" fill="#fff"/>
      <path d="M -3.6 -2 v -2.6 a 3.6 3.6 0 0 1 7.2 0 v 2.6" fill="none" stroke="#fff" stroke-width="1.8"/>
    </g>
  </g>
  </g>
</svg>
'''


if __name__ == "__main__":
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, "w") as f:
        f.write(svg())
    print(OUT, os.path.getsize(OUT), "bytes")
