/**
 * Curved section divider, echoing the wave breaks on the design reference
 * (laundrydry.com). `flip` mirrors it vertically so it can close a section
 * as well as open one. `color` is any Tailwind fill-* class.
 */
export function WaveDivider({
  color = "fill-white",
  flip = false,
  className = "",
}: {
  color?: string;
  flip?: boolean;
  className?: string;
}) {
  return (
    <div
      aria-hidden
      className={`pointer-events-none w-full overflow-hidden leading-[0] ${flip ? "rotate-180" : ""} ${className}`}
    >
      <svg
        viewBox="0 0 1440 100"
        preserveAspectRatio="none"
        className="h-16 w-full sm:h-24"
      >
        <path
          d="M0,40 C240,100 480,0 720,30 C960,60 1200,100 1440,40 L1440,100 L0,100 Z"
          className={color}
        />
      </svg>
    </div>
  );
}
