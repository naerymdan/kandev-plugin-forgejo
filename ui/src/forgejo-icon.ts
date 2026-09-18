import type { PluginHostApi, PluginIcon } from "@kandev/plugin-sdk";

/**
 * The Forgejo mark, drawn with the host's React so it inherits theme colors.
 * `currentColor` keeps it correct in both light and dark themes and inside the
 * host's semantic status colors.
 */
export function createForgejoIcon(host: PluginHostApi): PluginIcon {
  return function ForgejoIcon({ className }: { className?: string } = {}) {
    return host.jsx(
      "svg",
      {
        className,
        viewBox: "0 0 24 24",
        width: "1em",
        height: "1em",
        fill: "none",
        stroke: "currentColor",
        strokeWidth: 2,
        strokeLinecap: "round",
        strokeLinejoin: "round",
        "aria-hidden": "true",
        focusable: "false",
      },
      host.jsx("circle", { cx: 6, cy: 5, r: 2.5 }),
      host.jsx("circle", { cx: 18, cy: 5, r: 2.5 }),
      host.jsx("circle", { cx: 6, cy: 19, r: 2.5 }),
      host.jsx("path", { d: "M6 7.5v9" }),
      host.jsx("path", { d: "M18 7.5v2a4 4 0 0 1-4 4h-4" }),
    );
  };
}
