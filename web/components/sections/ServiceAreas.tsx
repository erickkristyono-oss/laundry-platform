import { Container } from "@/components/ui/Container";
import { site } from "@/lib/site";

export function ServiceAreas() {
  return (
    <section id="outlet" className="bg-slate-50 py-24">
      <Container className="text-center">
        <span className="text-sm font-semibold uppercase tracking-wide text-brand-600">
          Jaringan Outlet
        </span>
        <h2 className="mt-3 font-heading text-3xl font-bold text-navy-900 sm:text-4xl">
          Melayani Area Anda
        </h2>
        <p className="mx-auto mt-4 max-w-xl text-slate-600">
          Cakupan penjemputan &amp; pengantaran kami terus berkembang. Pilih
          outlet terdekat saat membuat pesanan.
        </p>

        <div className="mt-10 flex flex-wrap justify-center gap-3">
          {site.serviceAreas.map((area) => (
            <span
              key={area}
              className="rounded-full border border-brand-200 bg-white px-5 py-2 text-sm font-medium text-brand-700 shadow-sm"
            >
              {area}
            </span>
          ))}
        </div>
      </Container>
    </section>
  );
}
