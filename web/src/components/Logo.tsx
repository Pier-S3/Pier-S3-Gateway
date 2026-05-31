interface Props {
  /** Pixel size of the square icon. */
  size?: number;
  className?: string;
}

/**
 * Pier S3 brand mark.
 *
 * A monoline icon: a parcel/container (an object in storage) resting on a pier
 * deck with pilings, above a single wave. It uses `currentColor` for every
 * stroke, so it inherits the surrounding text/accent color and adapts to the
 * active theme (light/dark/preset) automatically.
 */
export default function Logo({ size = 24, className }: Props) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.7}
      strokeLinecap="round"
      strokeLinejoin="round"
      role="img"
      aria-hidden="true"
      className={className}
    >
      {/* Parcel / container on the deck */}
      <rect x="7.5" y="2.75" width="9" height="7" rx="1.2" />
      {/* Parcel tape: cross strap */}
      <path d="M12 2.75v7M7.5 6.25h9" />
      {/* Pier deck */}
      <path d="M3.5 12.4h17" />
      {/* Pilings down into the water */}
      <path d="M7 12.4v4.2M12 12.4v4.2M17 12.4v4.2" />
      {/* Wave - spans exactly the pier deck width (x 3.5 → 20.5) */}
      <path d="M3.5 19.4q2.125 -1.6 4.25 0t4.25 0 4.25 0 4.25 0" />
    </svg>
  );
}
