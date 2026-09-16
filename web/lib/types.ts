// Mirrors the JSON shapes returned by Core Service (docs/07-api-contract.md §6, §8).

export type Outlet = {
  id: string;
  code: string;
  name: string;
  address: string;
  phone?: string;
  is_active: boolean;
};

export type ServiceCatalogItem = {
  id: string;
  code: string;
  name: string;
  unit: "kg" | "pcs";
  is_active: boolean;
  price_per_unit?: number;
};

export type OrderItem = {
  id: string;
  service_id: string;
  service_name_snapshot: string;
  unit: string;
  estimated_weight_kg?: string;
  actual_weight_kg?: string;
  unit_price_snapshot: number;
  subtotal?: number;
  is_locked: boolean;
};

export type OrderStatus =
  | "CREATED"
  | "RECEIVED"
  | "WEIGHING"
  | "WASHING"
  | "DRYING"
  | "IRONING"
  | "PACKING"
  | "READY"
  | "PICKED_UP"
  | "DELIVERED"
  | "COMPLETED"
  | "ON_HOLD"
  | "CANCELLED";

export type PaymentStatus = "UNPAID" | "PENDING" | "PAID" | "FAILED" | "REFUNDED";

export type Order = {
  id: string;
  order_code: string;
  customer_id: string;
  current_outlet_id: string;
  source: string;
  fulfillment_type: string;
  status: OrderStatus;
  payment_status: PaymentStatus;
  estimated_total_amount?: number;
  subtotal_amount?: number;
  discount_amount: number;
  tax_amount: number;
  tax_rate_snapshot: string;
  pickup_fee: number;
  delivery_fee: number;
  total_amount?: number;
  notes?: string;
  created_at: string;
  items: OrderItem[];
};

export type Payment = {
  id: string;
  payment_code: string;
  order_id: string;
  customer_id: string;
  amount: number;
  method: "CASH" | "QRIS" | "TRANSFER" | "CARD";
  status: "PENDING" | "PAID" | "FAILED" | "REFUNDED";
  provider: string;
  checkout_url?: string;
  qr_string?: string;
  paid_at?: string;
  created_at: string;
};

export const ORDER_STEPS: { status: OrderStatus; label: string }[] = [
  { status: "CREATED", label: "Dibuat" },
  { status: "RECEIVED", label: "Diterima" },
  { status: "WEIGHING", label: "Ditimbang" },
  { status: "WASHING", label: "Dicuci" },
  { status: "DRYING", label: "Dikeringkan" },
  { status: "IRONING", label: "Disetrika" },
  { status: "PACKING", label: "Dikemas" },
  { status: "READY", label: "Siap Diambil" },
  { status: "PICKED_UP", label: "Diambil" },
  { status: "COMPLETED", label: "Selesai" },
];

export function formatIDR(amount: number | undefined | null): string {
  if (amount === undefined || amount === null) return "—";
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(amount);
}
