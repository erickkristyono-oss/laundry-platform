import type { OrderStatus, PaymentStatus } from "@/lib/types";

const statusColor: Record<OrderStatus, string> = {
  CREATED: "bg-slate-100 text-slate-600",
  RECEIVED: "bg-brand-50 text-brand-700",
  WEIGHING: "bg-brand-50 text-brand-700",
  WASHING: "bg-blue-50 text-blue-700",
  DRYING: "bg-blue-50 text-blue-700",
  IRONING: "bg-blue-50 text-blue-700",
  PACKING: "bg-blue-50 text-blue-700",
  READY: "bg-amber-50 text-amber-700",
  PICKED_UP: "bg-green-50 text-green-700",
  DELIVERED: "bg-green-50 text-green-700",
  COMPLETED: "bg-green-100 text-green-800",
  ON_HOLD: "bg-orange-50 text-orange-700",
  CANCELLED: "bg-red-50 text-red-700",
};

const statusLabel: Record<OrderStatus, string> = {
  CREATED: "Dibuat",
  RECEIVED: "Diterima",
  WEIGHING: "Ditimbang",
  WASHING: "Dicuci",
  DRYING: "Dikeringkan",
  IRONING: "Disetrika",
  PACKING: "Dikemas",
  READY: "Siap Diambil",
  PICKED_UP: "Diambil",
  DELIVERED: "Diantar",
  COMPLETED: "Selesai",
  ON_HOLD: "Ditahan",
  CANCELLED: "Dibatalkan",
};

export function StatusBadge({ status }: { status: OrderStatus }) {
  return (
    <span className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold ${statusColor[status]}`}>
      {statusLabel[status]}
    </span>
  );
}

const paymentColor: Record<PaymentStatus, string> = {
  UNPAID: "bg-red-50 text-red-700",
  PENDING: "bg-amber-50 text-amber-700",
  PAID: "bg-green-50 text-green-700",
  FAILED: "bg-red-50 text-red-700",
  REFUNDED: "bg-slate-100 text-slate-600",
};

const paymentLabel: Record<PaymentStatus, string> = {
  UNPAID: "Belum Bayar",
  PENDING: "Menunggu",
  PAID: "Lunas",
  FAILED: "Gagal",
  REFUNDED: "Dikembalikan",
};

export function PaymentBadge({ status }: { status: PaymentStatus }) {
  return (
    <span className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold ${paymentColor[status]}`}>
      {paymentLabel[status]}
    </span>
  );
}
