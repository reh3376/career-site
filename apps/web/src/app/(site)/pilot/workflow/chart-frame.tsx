"use client";

import { useState } from "react";

type Zoom = "fit" | "half" | "full";

// The company-workflow chart in a scrolling frame with three sizes.
// The SVG is 2814 by 4329 pixels: at full size its text is legible but
// the frame opens on a blank corner of a very large picture, and fitted
// to the frame the whole shape is visible but the text is not. So it
// opens fitted, for the shape, and a participant zooms to read; the
// full-size link beside the frame opens the file itself.
//
// The markup is the repository's own committed file, read from disk on
// the server and handed down as a string, never anything a request
// supplied.
export function ChartFrame({ svg }: { svg: string }) {
  const [zoom, setZoom] = useState<Zoom>("fit");
  return (
    <div>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">size</span>
        {(["fit", "half", "full"] as const).map((z) => (
          <button
            key={z}
            type="button"
            onClick={() => setZoom(z)}
            aria-pressed={zoom === z}
            className={
              "inline-flex h-[26px] items-center border px-3 font-mono text-[11px] uppercase tracking-[0.14em] transition-colors " +
              (zoom === z ? "border-accent bg-accent text-white" : "border-line text-ink-2 hover:border-accent hover:text-accent")
            }
          >
            {z}
          </button>
        ))}
      </div>
      <div
        className="pilot-chart mt-3 max-h-[75vh] overflow-auto border border-line bg-white p-4"
        data-zoom={zoom}
        dangerouslySetInnerHTML={{ __html: svg }}
      />
      <p className="mt-2 text-xs text-ink-3">
        {zoom === "fit"
          ? "The whole chart, too small to read: choose half or full, then scroll inside the frame. It reads top to bottom, phase by phase."
          : "Scroll inside the frame to follow the chart; it reads top to bottom, phase by phase."}
      </p>
    </div>
  );
}
