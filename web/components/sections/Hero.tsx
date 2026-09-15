import { Container } from "@/components/ui/Container";
import { LinkButton } from "@/components/ui/Button";

const steps = [
  { label: "Diterima", done: true },
  { label: "Ditimbang", done: true },
  { label: "Dicuci", done: true },
  { label: "Siap Diambil", done: false },
];

export function Hero() {
  return (
    <section className="relative overflow-hidden bg-white pt-14 pb-20 sm:pt-20 sm:pb-28">
      <Container className="relative grid items-center gap-14 lg:grid-cols-2">
        <div>
          <span className="inline-flex items-center rounded-full bg-brand-50 px-4 py-1.5 text-xs font-semibold uppercase tracking-wide text-brand-700">
            Platform Laundry Multi-Outlet
          </span>

          <h1 className="mt-6 font-heading text-4xl font-bold leading-tight text-navy-900 sm:text-5xl">
            Cucian Numpuk?{" "}
            <span className="text-red-600">Serahkan Saja</span> ke Kami.
          </h1>

          <p className="mt-5 max-w-lg text-base leading-relaxed text-slate-600 sm:text-lg">
            Kami jemput, cuci, dan antar kembali — dengan status yang bisa
            Anda pantau langsung, harga yang transparan sejak awal, dan
            jaringan outlet yang siap melayani area Anda.
          </p>

          <div className="mt-8 flex flex-wrap gap-4">
            <LinkButton href="/register">Pesan Sekarang</LinkButton>
            <LinkButton href="/#cara-kerja" variant="outline" className="!border-brand-600 !text-brand-700 hover:!bg-brand-50 hover:!text-brand-700">
              Lihat Cara Kerja
            </LinkButton>
          </div>

          <dl className="mt-12 grid max-w-md grid-cols-3 gap-6 border-t border-slate-100 pt-8">
            <div>
              <dt className="text-2xl font-bold text-navy-900">30+</dt>
              <dd className="text-xs text-slate-500">Outlet Jaringan</dd>
            </div>
            <div>
              <dt className="text-2xl font-bold text-navy-900">24 Jam</dt>
              <dd className="text-xs text-slate-500">Estimasi Express</dd>
            </div>
            <div>
              <dt className="text-2xl font-bold text-navy-900">100%</dt>
              <dd className="text-xs text-slate-500">Harga Transparan</dd>
            </div>
          </dl>
        </div>

        {/* Product mockup card instead of a stock photo — shows the actual
            order-tracking concept from docs/10-state-machines.md §1. */}
        <div className="relative mx-auto w-full max-w-sm">
          <div className="absolute -inset-6 -z-10 rounded-[2rem] bg-brand-100/70 blur-2xl" />
          <div className="rounded-3xl border border-slate-100 bg-white p-6 shadow-2xl shadow-brand-900/10">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs text-slate-400">Order</p>
                <p className="font-heading text-lg font-bold text-navy-900">
                  ORD-20260915-000042
                </p>
              </div>
              <span className="rounded-full bg-brand-50 px-3 py-1 text-xs font-semibold text-brand-700">
                Dicuci
              </span>
            </div>

            <div className="mt-6 space-y-4">
              {steps.map((step, i) => (
                <div key={step.label} className="flex items-center gap-3">
                  <span
                    className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-[11px] font-bold ${
                      step.done
                        ? "bg-brand-600 text-white"
                        : "bg-slate-100 text-slate-400"
                    }`}
                  >
                    {step.done ? "✓" : i + 1}
                  </span>
                  <span
                    className={`text-sm ${step.done ? "text-navy-900 font-medium" : "text-slate-400"}`}
                  >
                    {step.label}
                  </span>
                </div>
              ))}
            </div>

            <div className="mt-6 flex items-center justify-between rounded-xl bg-slate-50 px-4 py-3">
              <span className="text-sm text-slate-500">Total Estimasi</span>
              <span className="font-heading font-bold text-navy-900">
                Rp 57.000
              </span>
            </div>
          </div>
        </div>
      </Container>
    </section>
  );
}
