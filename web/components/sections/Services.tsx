import Link from "next/link";
import { Container } from "@/components/ui/Container";

const services = [
  {
    name: "Cuci Kiloan",
    desc: "Wash & fold harian untuk pakaian sehari-hari, ditimbang dan dihitung transparan.",
    icon: <path d="M4 4h16v16H4z M4 10h16 M9 4v3a3 3 0 0 0 6 0V4" />,
    available: true,
  },
  {
    name: "Cuci Express",
    desc: "Butuh cepat? Selesai dalam hitungan jam untuk kebutuhan mendesak Anda.",
    icon: <path d="M13 2 3 14h7l-1 8 11-14h-7l0-6Z" />,
    available: false,
  },
  {
    name: "Dry Clean",
    desc: "Perawatan khusus untuk bahan sensitif dan pakaian formal kesayangan Anda.",
    icon: <path d="M12 3v2M6 8h12l2 3v8a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-8l2-3Z M9 13a3 3 0 0 0 6 0" />,
    available: false,
  },
  {
    name: "Sepatu",
    desc: "Deep-clean untuk sepatu favorit Anda agar kembali bersih dan segar.",
    icon: <path d="M3 18v-2c3-1 4-4 7-4h2l7 3v3a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1Z M10 12V8a2 2 0 0 1 2-2" />,
    available: false,
  },
  {
    name: "Boneka",
    desc: "Solusi perawatan boneka kesayangan agar terhindar dari bakteri dan bau apek.",
    icon: <circle cx="12" cy="9" r="4" />,
    available: false,
  },
  {
    name: "Karpet & Gordyn",
    desc: "Cara ampuh hilangkan debu dan bau di karpet serta gorden rumah Anda.",
    icon: <path d="M4 4h16v16H4z M4 9h16 M4 14h16" />,
    available: false,
  },
];

export function Services() {
  return (
    <section id="layanan" className="bg-white py-24">
      <Container>
        <div className="mx-auto max-w-2xl text-center">
          <span className="text-sm font-semibold uppercase tracking-wide text-brand-600">
            Layanan Kami
          </span>
          <h2 className="mt-3 font-heading text-3xl font-bold text-navy-900 sm:text-4xl">
            Satu Platform, Semua Kebutuhan Laundry
          </h2>
          <p className="mt-4 text-slate-600">
            Kami menyediakan beberapa layanan untuk menjaga kualitas pakaian
            Anda tetap terjaga, dikerjakan oleh staf terlatih di outlet
            terdekat.
          </p>
        </div>

        <div className="mt-14 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {services.map((service) => (
            <Link
              key={service.name}
              href="/order/new"
              className="group relative rounded-2xl border border-slate-100 bg-white p-7 text-center shadow-sm shadow-slate-900/5 transition-shadow hover:shadow-md"
            >
              {!service.available && (
                <span className="absolute right-4 top-4 rounded-full bg-slate-100 px-2.5 py-1 text-[11px] font-semibold text-slate-500">
                  Segera Hadir
                </span>
              )}
              <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-brand-500 text-white transition-transform group-hover:scale-105">
                <svg
                  viewBox="0 0 24 24"
                  className="h-7 w-7"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  {service.icon}
                </svg>
              </div>
              <h3 className="mt-5 font-heading text-lg font-semibold text-brand-700">
                {service.name}
              </h3>
              <p className="mt-2 text-sm leading-relaxed text-slate-500">
                {service.desc}
              </p>
              <p className="mt-3 text-xs font-semibold text-brand-600 group-hover:underline">
                {service.available ? "Pesan sekarang →" : "Pesan Cuci Kiloan dulu →"}
              </p>
            </Link>
          ))}
        </div>
      </Container>
    </section>
  );
}
