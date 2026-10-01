#!/usr/bin/env python3
"""Pull the text out of better-business-decisions-part3.pdf.

The PDF has no /Font object and no ToUnicode map, so nothing off the
shelf reads it. What it does have is a subset font whose glyph ids are
ASCII shifted by a constant: <0023> is 'B', 0x23 + 0x1F == 0x42. That
holds across every stream, so the decode is a shift, not a guess.

Word breaks are kerning, not spaces: the operands between hex strings in
a TJ array are negative tenths of an em, and a large one is a word gap.
Line breaks come from Td/TD/T* between BT/ET blocks.

Output is deliberately raw. It is a faithful dump for a human to
structure, not a Markdown conversion: inferring headings from font sizes
would be the step where a conversion quietly rewrites someone's article.
"""

import re
import sys
import zlib

SHIFT = 0x1F
# Below this, the gap between two runs is a word break rather than
# kerning. The file decides this, not taste: the kern values are
# bimodal, with 4,247 of them between -200 and -500 (word gaps) and 221
# between -50 and +258 (letter kerning), and not a single one in
# between. -100 sits in the empty band, so the split is unambiguous.
WORD_GAP = -100


def decode_hex(h: str) -> str:
    out = []
    for i in range(0, len(h), 4):
        code = int(h[i : i + 4], 16)
        out.append(chr(code + SHIFT))
    return "".join(out)


def text_from_stream(data: bytes) -> str:
    """Walk a content stream in order, honouring the cursor moves.

    The first version collected every TJ array in a BT..ET block and
    joined them with nothing, which silently welded the last word of one
    line to the first word of the next: "intended to improvehow". Inside
    one block, a Td/TD/T* with a vertical component is a new line, and a
    TJ that follows one needs a break in front of it. A purely
    horizontal move is a font or style change mid-line and must not
    introduce a space, or every italicised phrase gains one.
    """
    s = data.decode("latin-1")
    out = []
    for block in re.findall(r"BT(.*?)ET", s, re.S):
        line = []
        lines = []
        # TJ arrays, Tj strings and the cursor moves, in source order.
        token = re.compile(
            r"\[(?P<tj>.*?)\]\s*TJ"
            r"|<(?P<single>[0-9A-Fa-f]+)>\s*Tj"
            r"|(?P<x>-?[\d.]+)\s+(?P<y>-?[\d.]+)\s+T[dD]"
            r"|(?P<star>T\*)",
            re.S,
        )
        for m in token.finditer(block):
            if m.group("tj") is not None:
                buf = []
                for hx, num in re.findall(
                    r"<([0-9A-Fa-f]+)>|(-?\d+)", m.group("tj")
                ):
                    if hx:
                        buf.append(decode_hex(hx))
                    elif num and int(num) <= WORD_GAP:
                        buf.append(" ")
                line.append("".join(buf))
            elif m.group("single") is not None:
                line.append(decode_hex(m.group("single")))
            else:
                moved_down = m.group("star") is not None or (
                    m.group("y") is not None and float(m.group("y")) != 0
                )
                if moved_down and line:
                    lines.append("".join(line))
                    line = []
                elif m.group("y") is not None and float(m.group("y")) == 0:
                    # A horizontal-only move is a style change mid-line,
                    # and in this file the gap it jumps over is the word
                    # space: "...intended to improve" then /F1 and a Td
                    # that lands at "how". Dropping it welded the two
                    # together. A space goes in unless one is already
                    # there or the run ends mid-word on a hyphen, which
                    # is where emphasis legitimately starts without one.
                    so_far = "".join(line)
                    if so_far and not so_far.endswith((" ", "-", "(", "/")):
                        line.append(" ")
        if line:
            lines.append("".join(line))
        text = " ".join(x.strip() for x in lines if x.strip())
        if text:
            out.append(text)
    return "\n".join(out)


def main(path: str) -> None:
    raw = open(path, "rb").read()
    chunks = []
    for m in re.finditer(rb"stream\r?\n(.*?)endstream", raw, re.S):
        body = m.group(1)
        try:
            data = zlib.decompress(body.strip(b"\r\n"))
        except zlib.error:
            continue
        if b"BT" not in data:
            continue
        chunks.append(text_from_stream(data))
    print("\n\n<<<PAGE BREAK>>>\n\n".join(c for c in chunks if c.strip()))


if __name__ == "__main__":
    main(sys.argv[1])
