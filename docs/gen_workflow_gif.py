#!/usr/bin/env python3
"""Generates a fake animated GIF of the "navigate -> restore" workflow for
the README: snapshot list -> browse tree -> pick a file -> ctrl+r restore ->
set target path -> done. Synthetic data/frames, matching the real app's
layout/colors/text — not a live capture (see gen_screenshot.py for the
single static screenshot, whose styling this reuses)."""

import random
import datetime
from PIL import Image, ImageDraw

import gen_screenshot as gs

random.seed(7)

TITLEBAR_H = 36
IMG_W = 940  # wide enough for the longest help line at this font size
IMG_H = 480
HEATMAP_WEEKS = 26  # narrowed to fit IMG_W, same width-aware logic the real tool uses


def new_canvas():
    img = Image.new("RGB", (IMG_W, IMG_H), gs.BG)
    draw = ImageDraw.Draw(img)
    draw.rectangle([0, 0, IMG_W, TITLEBAR_H], fill=(28, 29, 38))
    for i, c in enumerate([(255, 95, 86), (255, 189, 46), (39, 201, 63)]):
        draw.ellipse([gs.PAD + i * 24, TITLEBAR_H / 2 - 7, gs.PAD + i * 24 + 14, TITLEBAR_H / 2 + 7], fill=c)
    label = "restic-tui — hhenrique@localhost"
    draw.text((IMG_W / 2 - len(label) * gs.CHAR_W / 2, TITLEBAR_H / 2 - gs.FONT_SIZE / 2),
               label, font=gs.font, fill=(150, 150, 160))
    return img, draw


def draw_rows(draw, rows, y):
    for row in rows:
        gs.draw_segments(draw, gs.PAD, y, row)
        y += gs.LINE_H
    return y


def frame_snapshot_list(snaps, cursor_idx):
    img, draw = new_canvas()
    y = gs.PAD + TITLEBAR_H
    gs.draw_segments(draw, gs.PAD, y, [("Snapshots", gs.TITLE)], bold=True)
    y += gs.LINE_H * 2
    # narrower heatmap (fewer recent weeks) so it fits this GIF's compact width,
    # same width-aware narrowing the real tool does for small terminals
    hm_rows = gs.heatmap_lines(snaps, datetime.date.today(), weeks=HEATMAP_WEEKS)
    y = draw_rows(draw, hm_rows, y)
    y += gs.LINE_H
    rows = gs.snapshot_list_lines(snaps, cursor_idx=cursor_idx, max_rows=8)
    y = draw_rows(draw, rows, y)
    y += gs.LINE_H
    gs.draw_segments(draw, gs.PAD, y, [
        ("↑/↓ move · enter browse · ctrl+f forget · ctrl+r refresh · ctrl+q quit", gs.HELP)
    ])
    return img


def frame_tree(short_id, breadcrumbs, esc_target, nodes, cursor_idx):
    img, draw = new_canvas()
    y = gs.PAD + TITLEBAR_H
    crumbs = " / ".join(breadcrumbs)
    gs.draw_segments(draw, gs.PAD, y, [
        ("Snapshot " + short_id, gs.TITLE), ("  " + crumbs, gs.DIM)
    ], bold=False)
    y += gs.LINE_H
    gs.draw_segments(draw, gs.PAD, y, [("esc → " + esc_target, gs.DIM)])
    y += gs.LINE_H * 2
    for i, (name, kind) in enumerate(nodes):
        # Nerd Font glyphs (this font ships them in its private-use area),
        # not emoji — PIL/a single monospace font can't render color emoji,
        # whereas a real terminal emulator renders the actual app's emoji
        # fine via font fallback.
        icon = "" if kind == "dir" else ""
        text = "%s %s" % (icon, name)
        if i == cursor_idx:
            gs.draw_segments(draw, gs.PAD, y, [("> " + text, gs.SELECTED)])
        else:
            color = (90, 160, 255) if kind == "dir" else gs.FG
            gs.draw_segments(draw, gs.PAD, y, [("  " + text, color)])
        y += gs.LINE_H
    y += gs.LINE_H
    gs.draw_segments(draw, gs.PAD, y, [
        ("↑/↓ move · type to jump · enter open/select · ctrl+r restore · esc back · ctrl+q quit", gs.HELP)
    ])
    return img


def frame_restore_prompt(source, target, cursor_block=True):
    img, draw = new_canvas()
    y = gs.PAD + TITLEBAR_H
    gs.draw_segments(draw, gs.PAD, y, [("Restore", gs.TITLE)], bold=True)
    y += gs.LINE_H * 2
    gs.draw_segments(draw, gs.PAD, y, [("Source: " + source, gs.FG)])
    y += gs.LINE_H
    gs.draw_segments(draw, gs.PAD, y, [("Target path:", gs.FG)])
    y += gs.LINE_H
    cursor = "█" if cursor_block else ""
    gs.draw_segments(draw, gs.PAD, y, [(target + cursor, gs.SELECTED)])
    y += gs.LINE_H * 2
    gs.draw_segments(draw, gs.PAD, y, [("enter confirm · esc cancel", gs.HELP)])
    return img


def frame_message(lines, help_text):
    img, draw = new_canvas()
    y = gs.PAD + TITLEBAR_H
    for line, color in lines:
        gs.draw_segments(draw, gs.PAD, y, [(line, color)])
        y += gs.LINE_H
    y += gs.LINE_H
    gs.draw_segments(draw, gs.PAD, y, [(help_text, gs.HELP)])
    return img


def main():
    today = datetime.date.today()
    snaps = gs.build_fake_snapshots(today)
    snap = snaps[2]  # "down two" from the top

    root_nodes = [
        (".config", "dir"), (".ssh", "dir"), ("Documents", "dir"),
        ("Downloads", "dir"), ("git", "dir"), ("Pictures", "dir"),
        (".bashrc", "file"), (".gitconfig", "file"),
    ]
    git_nodes = [("hosted", "dir"), ("sandbox", "dir"), ("work-in-progress", "dir")]
    sandbox_nodes = [
        ("restic-tui", "dir"),
        ("notes.md", "file"), ("README.md", "file"),
    ]

    source_path = "/home/hhenrique/git/sandbox/README.md"
    target_default = "/home/hhenrique/restic-restores/README.md"
    target_final = "/home/hhenrique/Desktop/README.md"

    frames = []
    durations = []

    def add(img, ms):
        frames.append(img.convert("P", palette=Image.ADAPTIVE))
        durations.append(ms)

    add(frame_snapshot_list(snaps, cursor_idx=0), 1400)
    add(frame_snapshot_list(snaps, cursor_idx=1), 500)
    add(frame_snapshot_list(snaps, cursor_idx=2), 1400)

    add(frame_tree(snap["short_id"], ["/home/hhenrique"], "snapshot list", root_nodes, cursor_idx=4), 1400)
    add(frame_tree(snap["short_id"], ["/home/hhenrique", "git"], "/home/hhenrique", git_nodes, cursor_idx=1), 1400)
    add(frame_tree(snap["short_id"], ["/home/hhenrique", "git", "sandbox"], "git", sandbox_nodes, cursor_idx=2), 1600)

    add(frame_restore_prompt(source_path, target_default), 1300)
    add(frame_restore_prompt(source_path, target_final), 1500)

    add(frame_message(
        [("Restoring " + source_path + " -> " + target_final + " ...", gs.FG)],
        "please wait...",
    ), 1000)
    add(frame_message(
        [("Restore complete.", (80, 250, 123)), ("", gs.FG)],
        "enter/esc/q back to snapshots · ctrl+q quit",
    ), 2200)

    frames[0].save(
        "./docs/workflow.gif",
        save_all=True,
        append_images=frames[1:],
        duration=durations,
        loop=0,
        optimize=True,
    )
    print("saved", len(frames), "frames ->", "docs/workflow.gif")


if __name__ == "__main__":
    main()
