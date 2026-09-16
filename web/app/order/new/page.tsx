"use client";

import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Container } from "@/components/ui/Container";
import { Button } from "@/components/ui/Button";
import { Field, Input } from "@/components/ui/Input";
import { useCustomerSession } from "@/lib/useSession";
import { apiFetch, ApiError } from "@/lib/api";
import { formatIDR, type Order, type Outlet, type ServiceCatalogItem } from "@/lib/types";

export default function NewOrderPage() {
  const router = useRouter();
  const session = useCustomerSession();

  const [services, setServices] = useState<ServiceCatalogItem[] | null>(null);
  const [serviceId, setServiceId] = useState("");
  const [outlets, setOutlets] = useState<Outlet[] | null>(null);
  const [outletId, setOutletId] = useState("");
  const [weightKg, setWeightKg] = useState("3");
  const [notes, setNotes] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<{ data: ServiceCatalogItem[] }>("/api/v1/services", { auth: "none" })
      .then((res) => {
        setServices(res.data ?? []);
        if (res.data?.[0]) setServiceId(res.data[0].id);
      })
      .catch(() => setError("Tidak dapat memuat daftar layanan"));

    apiFetch<{ data: Outlet[] }>("/api/v1/outlets", { auth: "none" })
      .then((res) => {
        setOutlets(res.data ?? []);
        if (res.data?.[0]) setOutletId(res.data[0].id);
      })
      .catch(() => setError("Tidak dapat memuat daftar outlet."));
  }, []);

  if (!session) return null;

  const selectedService = services?.find((s) => s.id === serviceId);
  const estimate =
    selectedService?.price_per_unit && weightKg
      ? Number(weightKg) * selectedService.price_per_unit
      : undefined;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!session || !serviceId) return;
    setLoading(true);
    setError(null);
    try {
      const order = await apiFetch<Order>("/api/v1/orders", {
        method: "POST",
        body: JSON.stringify({
          customer: { id: session.customer.id },
          outlet_id: outletId || undefined,
          source: "WEBSITE",
          fulfillment_type: "WALK_IN",
          items: [{ service_id: serviceId, estimated_weight_kg: Number(weightKg) }],
          notes: notes || undefined,
        }),
      });
      router.push(`/orders/${order.id}`);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : "Terjadi kesalahan, coba lagi.",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="bg-slate-50 py-12">
      <Container className="max-w-xl">
        <h1 className="font-heading text-2xl font-bold text-navy-900">
          Buat Pesanan Baru
        </h1>
        <p className="mt-1 text-sm text-slate-500">
          Berat aktual akan ditimbang ulang oleh staf outlet saat pesanan diterima.
        </p>

        <form onSubmit={onSubmit} className="mt-8 space-y-5 rounded-2xl border border-slate-100 bg-white p-6 shadow-sm">
          <Field label="Outlet">
            <select
              required
              value={outletId}
              onChange={(e) => setOutletId(e.target.value)}
              className="w-full rounded-xl border border-slate-200 px-4 py-2.5 text-sm text-slate-900 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            >
              {outlets?.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name} — {o.address}
                </option>
              ))}
            </select>
          </Field>

          <Field label="Layanan">
            <select
              required
              value={serviceId}
              onChange={(e) => setServiceId(e.target.value)}
              className="w-full rounded-xl border border-slate-200 px-4 py-2.5 text-sm text-slate-900 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            >
              {services?.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} {s.price_per_unit ? `— ${formatIDR(s.price_per_unit)}/${s.unit}` : ""}
                </option>
              ))}
            </select>
          </Field>

          <Field label="Estimasi Berat (kg)">
            <Input
              type="number"
              min="0.5"
              step="0.5"
              required
              value={weightKg}
              onChange={(e) => setWeightKg(e.target.value)}
            />
          </Field>

          <Field label="Catatan (opsional)">
            <Input value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Contoh: jangan pakai pewangi" />
          </Field>

          {estimate !== undefined && (
            <div className="flex items-center justify-between rounded-xl bg-brand-50 px-4 py-3 text-sm">
              <span className="text-brand-800">Estimasi Biaya</span>
              <span className="font-semibold text-brand-800">{formatIDR(estimate)}</span>
            </div>
          )}

          {error && (
            <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
          )}

          <Button type="submit" disabled={loading || !serviceId || !outletId} className="w-full">
            {loading ? "Memproses…" : "Buat Pesanan"}
          </Button>
        </form>
      </Container>
    </div>
  );
}
