#!/usr/bin/env python3
"""Generates a fake terminal screenshot of restic-tui's initial screen
(heatmap + snapshot list) for the README. Not a real screenshot — synthetic
data, rendered to match the actual app's layout/colors/text exactly."""

import random
import datetime
from PIL import Image, ImageDraw, ImageFont

random.seed(42)

FONT_REGULAR = "/usr/share/fonts/TTF/JetBrainsMonoNerdFontMono-Regular.ttf"
FONT_BOLD = "/usr/share/fonts/TTF/JetBrainsMonoNerdFontMono-Bold.ttf"
FONT_SIZE = 16
CHAR_W = 9.6
LINE_H = 20
PAD = 18

BG = (13, 14, 20)
FG = (224, 224, 224)
TITLE = (255, 175, 0)
DIM = (168, 168, 168)
HELP = (188, 188, 188)
SELECTED = (80, 250, 123)
LEVELS = [
    (90, 90, 90),   # 0 no backup -> rendered as dim '·'
    (0, 95, 0),     # 1
    (0, 135, 0),    # 2
    (0, 175, 0),    # 3
    (0, 215, 0),    # 4
]

font = ImageFont.truetype(FONT_REGULAR, FONT_SIZE)
font_bold = ImageFont.truetype(FONT_BOLD, FONT_SIZE)

COLUMN_WIDTH = 3  # matches heatmap.go's columnWidth
WEEKS = 52


def bucket_for_bytes(b):
    if b == 0:
        return 0
    if b < 50 * 1024 * 1024:
        return 1
    if b < 500 * 1024 * 1024:
        return 2
    if b < 5 * 1024 * 1024 * 1024:
        return 3
    return 4


def build_fake_snapshots(today):
    """Simulate ~10 months of mostly-daily backups with realistic gaps/spikes."""
    snaps = []
    start_day = today - datetime.timedelta(days=WEEKS * 7 - 10)
    d = start_day
    first = True
    while d <= today:
        skip_chance = 0.12
        if not first and random.random() < skip_chance:
            d += datetime.timedelta(days=1)
            continue
        if first:
            data_added = 340 * 1024 * 1024 * 1024  # big initial backup
            first = False
        else:
            r = random.random()
            if r < 0.7:
                data_added = random.randint(5, 80) * 1024 * 1024
            elif r < 0.92:
                data_added = random.randint(100, 450) * 1024 * 1024
            else:
                data_added = random.randint(1, 4) * 1024 * 1024 * 1024
        hh = random.randint(0, 23)
        mm = random.randint(0, 59)
        ss = random.randint(0, 59)
        snaps.append({
            "short_id": "%08x" % random.randrange(16**8),
            "time": datetime.datetime(d.year, d.month, d.day, hh, mm, ss),
            "paths": ["/home/hhenrique"],
            "data_added": data_added,
        })
        d += datetime.timedelta(days=1)
    snaps.sort(key=lambda s: s["time"], reverse=True)
    return snaps


def heatmap_lines(snaps, today, weeks=WEEKS):
    """weeks controls how many of the most recent weeks are shown — mirrors
    the real Go tool narrowing the grid to fit a smaller terminal width,
    rather than clipping a fixed-size grid against a smaller canvas."""
    by_day = {}
    for s in snaps:
        k = s["time"].date()
        by_day[k] = by_day.get(k, 0) + s["data_added"]

    weekday = today.weekday()  # Mon=0..Sun=6
    # convert to Sun=0..Sat=6 convention used by the real tool
    today_sun0 = (weekday + 1) % 7
    week_end = today + datetime.timedelta(days=6 - today_sun0)
    start = week_end - datetime.timedelta(days=weeks * 7 - 1)

    month_label = [""] * weeks
    last_month = ""
    for c in range(weeks):
        col_start = start + datetime.timedelta(days=c * 7)
        for dd in range(7):
            day = col_start + datetime.timedelta(days=dd)
            m = day.strftime("%b")
            if day.day == 1 and m != last_month:
                month_label[c] = m
                last_month = m
                break

    rows = []
    header = [("    ", FG)]
    for c in range(weeks):
        label = month_label[c]
        header.append(("%-3s" % label, DIM))
    rows.append(header)

    weekday_labels = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"]
    for row in range(7):
        line = [("%-3s " % weekday_labels[row], FG)]
        for c in range(weeks):
            day = start + datetime.timedelta(days=c * 7 + row)
            if day > today:
                line.append(("   ", FG))
                continue
            bytes_added = by_day.get(day, 0)
            level = bucket_for_bytes(bytes_added)
            glyph = "■" if level > 0 else "·"
            line.append((glyph, LEVELS[level]))
            line.append(("  ", FG))
        rows.append(line)
    return rows


def snapshot_list_lines(snaps, cursor_idx, max_rows):
    lines = []
    visible = snaps[:max_rows]
    for i, s in enumerate(visible):
        ts = s["time"].strftime("%Y-%m-%d %H:%M:%S")
        text = "%s  %s  %s" % (s["short_id"], ts, ", ".join(s["paths"]))
        if i == cursor_idx:
            lines.append([("> " + text, SELECTED)])
        else:
            lines.append([("  " + text, FG)])
    if len(snaps) > max_rows:
        lines.append([("  ↓ %d more below" % (len(snaps) - max_rows), DIM)])
    return lines


def draw_segments(draw, x, y, segments, bold=False):
    f = font_bold if bold else font
    cx = x
    for text, color in segments:
        draw.text((cx, y), text, font=f, fill=color)
        cx += len(text) * CHAR_W


def main():
    today = datetime.date.today()
    snaps = build_fake_snapshots(today)

    hm_rows = heatmap_lines(snaps, today)
    snap_rows = snapshot_list_lines(snaps, cursor_idx=0, max_rows=14)

    width_chars = 4 + WEEKS * COLUMN_WIDTH
    img_w = int(PAD * 2 + width_chars * CHAR_W)
    total_lines = 1 + 1 + len(hm_rows) + 1 + len(snap_rows) + 2
    titlebar_h = 36
    img_h = int(PAD * 2 + total_lines * LINE_H) + titlebar_h

    img = Image.new("RGB", (img_w, img_h), BG)
    draw = ImageDraw.Draw(img)

    # fake terminal window chrome
    draw.rectangle([0, 0, img_w, titlebar_h], fill=(28, 29, 38))
    for i, c in enumerate([(255, 95, 86), (255, 189, 46), (39, 201, 63)]):
        draw.ellipse([PAD + i * 24, titlebar_h / 2 - 7, PAD + i * 24 + 14, titlebar_h / 2 + 7], fill=c)
    label = "restic-tui — hhenrique@localhost"
    draw.text((img_w / 2 - len(label) * CHAR_W / 2, titlebar_h / 2 - FONT_SIZE / 2), label, font=font, fill=(150, 150, 160))

    y = PAD + titlebar_h
    draw_segments(draw, PAD, y, [("Snapshots", TITLE)], bold=True)
    y += LINE_H * 2

    for row in hm_rows:
        draw_segments(draw, PAD, y, row)
        y += LINE_H
    y += LINE_H

    for row in snap_rows:
        draw_segments(draw, PAD, y, row)
        y += LINE_H
    y += LINE_H

    draw_segments(draw, PAD, y, [
        ("↑/↓ move · enter browse · ctrl+f forget · ctrl+r refresh · ctrl+q quit", HELP)
    ])

    img.save("./docs/screenshot.png")
    print("saved", img.size)


if __name__ == "__main__":
    main()
