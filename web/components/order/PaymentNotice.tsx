import type { PaymentStatus } from "@/lib/types";

const copy: Record<PaymentStatus, { title: string; body: string; tone: string }> = {
  UNPAID: {
    title: "Belum dibayar",
    body: "Total di atas masih estimasi. Setelah staf outlet selesai menimbang cucian Anda, pilihan bayar online (QRIS/transfer/kartu) akan muncul di halaman ini — atau bayar tunai langsung di outlet.",
    tone: "bg-amber-50 text-amber-800 border-amber-100",
  },
  PENDING: {
    title: "Menunggu konfirmasi pembayaran",
    body: "Pembayaran Anda sedang diproses. Halaman ini akan otomatis memperbarui status begitu pembayaran dikonfirmasi.",
    tone: "bg-blue-50 text-blue-800 border-blue-100",
  },
  PAID: {
    title: "Sudah lunas",
    body: "Terima kasih, pembayaran untuk pesanan ini sudah kami terima.",
    tone: "bg-green-50 text-green-800 border-green-100",
  },
  FAILED: {
    title: "Pembayaran gagal",
    body: "Pembayaran terakhir untuk pesanan ini gagal diproses. Silakan hubungi outlet untuk membayar tunai atau mencoba metode lain.",
    tone: "bg-red-50 text-red-800 border-red-100",
  },
  REFUNDED: {
    title: "Dana dikembalikan",
    body: "Pembayaran untuk pesanan ini sudah dikembalikan (refund).",
    tone: "bg-slate-50 text-slate-700 border-slate-200",
  },
};

export function PaymentNotice({ status }: { status: PaymentStatus }) {
  const c = copy[status];
  return (
    <div className={`mt-6 rounded-xl border px-4 py-3 text-sm ${c.tone}`}>
      <p className="font-semibold">{c.title}</p>
      <p className="mt-0.5">{c.body}</p>
    </div>
  );
}
