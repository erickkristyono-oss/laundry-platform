import { ORDER_STEPS, type OrderStatus } from "@/lib/types";

/** Linear progress timeline for the happy-path statuses
 * (docs/10-state-machines.md §1) — ON_HOLD/CANCELLED are shown separately
 * by the caller since they branch off this line rather than sitting on it. */
export function StatusTimeline({ status }: { status: OrderStatus }) {
  const currentIndex = ORDER_STEPS.findIndex((s) => s.status === status);

  if (status === "ON_HOLD" || status === "CANCELLED") {
    return null;
  }

  return (
    <ol className="flex flex-wrap gap-y-4">
      {ORDER_STEPS.map((step, i) => {
        const done = currentIndex >= 0 && i <= currentIndex;
        const isLast = i === ORDER_STEPS.length - 1;
        return (
          <li key={step.status} className="flex items-center">
            <div className="flex flex-col items-center gap-1.5">
              <span
                className={`flex h-7 w-7 items-center justify-center rounded-full text-[11px] font-bold ${
                  done ? "bg-brand-600 text-white" : "bg-slate-100 text-slate-400"
                }`}
              >
                {done ? "✓" : i + 1}
              </span>
              <span className={`w-20 text-center text-[11px] ${done ? "font-medium text-navy-900" : "text-slate-400"}`}>
                {step.label}
              </span>
            </div>
            {!isLast && (
              <span className={`mx-1 mb-5 h-px w-6 sm:w-10 ${done ? "bg-brand-400" : "bg-slate-200"}`} />
            )}
          </li>
        );
      })}
    </ol>
  );
}
