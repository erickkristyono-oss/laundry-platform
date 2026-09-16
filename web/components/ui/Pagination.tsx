import { Button } from "@/components/ui/Button";

export function Pagination({
  page,
  pageSize,
  total,
  onPageChange,
}: {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  // Shown even at 1 page (buttons disabled) rather than hidden — a hidden
  // control and a control that legitimately has nowhere to go look
  // identical to a user, so this makes "nothing to page through yet"
  // visibly true instead of ambiguous with "pagination is broken".
  if (total === 0) return null;

  return (
    <div className="mt-6 flex items-center justify-center gap-3">
      <Button
        type="button"
        variant="ghost"
        className="border border-slate-200"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
      >
        ← Sebelumnya
      </Button>
      <span className="text-sm text-slate-500">
        Halaman {page} dari {totalPages}
      </span>
      <Button
        type="button"
        variant="ghost"
        className="border border-slate-200"
        disabled={page >= totalPages}
        onClick={() => onPageChange(page + 1)}
      >
        Selanjutnya →
      </Button>
    </div>
  );
}
