/**
 * Inline stroke icons.
 *
 * Icons are always decorative here — every one is paired with a visible text
 * label in the markup — so they are marked `aria-hidden` and removed from the
 * accessibility tree. Nothing on this page depends on an icon to convey
 * meaning.
 */

import type { ReactElement } from "react";

export type IconName =
  | "server"
  | "layers"
  | "leaf"
  | "wind"
  | "tools"
  | "cursor"
  | "target"
  | "display"
  | "bolt"
  | "users"
  | "globe"
  | "refresh"
  | "alert"
  | "arrow";

const paths: Record<IconName, ReactElement> = {
  server: (
    <>
      <rect x="3" y="4" width="18" height="7" rx="2" />
      <rect x="3" y="13" width="18" height="7" rx="2" />
      <path d="M7 7.5h.01M7 16.5h.01" />
    </>
  ),
  layers: (
    <>
      <path d="M12 3 3 7.5l9 4.5 9-4.5L12 3Z" />
      <path d="M3 12.5 12 17l9-4.5" />
      <path d="M3 17 12 21.5 21 17" />
    </>
  ),
  leaf: (
    <>
      <path d="M4 20c0-8 5-13 16-14 0 11-5 15-11 15-3 0-5-1-5-1Z" />
      <path d="M4 20c4-5 8-8 13-10" />
    </>
  ),
  wind: (
    <>
      <path d="M3 8h11a3 3 0 1 0-3-3" />
      <path d="M3 16h8a3 3 0 1 1-3 3" />
      <path d="M3 12h18" />
    </>
  ),
  tools: (
    <>
      <path d="M14.5 6.5 17 4l3 3-2.5 2.5" />
      <path d="M17.5 9.5 8 19H4v-4l9.5-9.5" />
      <path d="M11 8l5 5" />
    </>
  ),
  cursor: (
    <>
      <path d="M5 3l6 17 2.5-6.5L20 11 5 3Z" />
    </>
  ),
  target: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <circle cx="12" cy="12" r="4.5" />
      <circle cx="12" cy="12" r="0.8" />
    </>
  ),
  display: (
    <>
      <rect x="2.5" y="4" width="19" height="13" rx="2" />
      <path d="M8.5 21h7M12 17v4" />
    </>
  ),
  bolt: (
    <>
      <path d="M13.5 2 5 13.5h5L9.5 22 19 10h-5.5l0-8Z" />
    </>
  ),
  users: (
    <>
      <circle cx="9" cy="8" r="3.5" />
      <path d="M2.5 20a6.5 6.5 0 0 1 13 0" />
      <path d="M16 5.2a3.5 3.5 0 0 1 0 5.6" />
      <path d="M18 14.6a6.5 6.5 0 0 1 3.5 5.4" />
    </>
  ),
  globe: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18" />
      <path d="M12 3c3 3.5 3 14.5 0 18-3-3.5-3-14.5 0-18Z" />
    </>
  ),
  refresh: (
    <>
      <path d="M20 12a8 8 0 1 1-2.6-5.9" />
      <path d="M20 3.5V8h-4.5" />
    </>
  ),
  alert: (
    <>
      <path d="M12 3.5 21 19H3l9-15.5Z" />
      <path d="M12 9.5v4M12 16.5h.01" />
    </>
  ),
  arrow: (
    <>
      <path d="M4 12h15" />
      <path d="M13.5 6.5 20 12l-6.5 5.5" />
    </>
  ),
};

interface IconProps {
  name: IconName;
  /** Rendered size in pixels. Defaults to 1em-ish at 20px. */
  size?: number;
  className?: string;
}

export default function Icon({ name, size = 20, className }: IconProps) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {paths[name]}
    </svg>
  );
}
