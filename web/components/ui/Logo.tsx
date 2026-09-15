import { site } from "@/lib/site";

export function Logo({ inverted = false }: { inverted?: boolean }) {
  return (
    <span className="inline-flex items-center gap-2 font-heading text-xl font-bold tracking-tight">
      <svg
        viewBox="0 0 40 40"
        className="h-9 w-9"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <circle
          cx="20"
          cy="20"
          r="19"
          className={inverted ? "fill-white/10" : "fill-brand-50"}
          stroke="currentColor"
          strokeWidth="2"
          strokeOpacity="0.15"
        />
        <circle cx="20" cy="20" r="12" className="fill-brand-500" />
        <path
          d="M14 20a6 6 0 0 1 10.5-4M26 20a6 6 0 0 1-10.5 4"
          stroke="white"
          strokeWidth="2.2"
          strokeLinecap="round"
        />
      </svg>
      <span className={inverted ? "text-white" : "text-navy-900"}>
        {site.name}
      </span>
    </span>
  );
}
