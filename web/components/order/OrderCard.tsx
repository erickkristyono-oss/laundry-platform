import Link from "next/link";
import { StatusBadge, PaymentBadge } from "./StatusBadge";
import { formatIDR, type Order } from "@/lib/types";

export function OrderCard({ order, href }: { order: Order; href: string }) {
  return (
    <Link
      href={href}
      className="block rounded-2xl border border-slate-100 bg-white p-5 shadow-sm shadow-slate-900/5 transition-shadow hover:shadow-md"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="font-heading font-semibold text-navy-900">{order.order_code}</p>
          <p className="mt-0.5 text-xs text-slate-500">
            {new Date(order.created_at).toLocaleString("id-ID")}
          </p>
        </div>
        <div className="flex gap-2">
          <StatusBadge status={order.status} />
          <PaymentBadge status={order.payment_status} />
        </div>
      </div>
      <div className="mt-3 flex items-center justify-between border-t border-slate-100 pt-3 text-sm">
        <span className="text-slate-500">{order.items.length} layanan</span>
        <span className="font-semibold text-navy-900">
          {formatIDR(order.total_amount ?? order.estimated_total_amount)}
        </span>
      </div>
    </Link>
  );
}
