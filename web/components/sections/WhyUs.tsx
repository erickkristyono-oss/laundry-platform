import { Container } from "@/components/ui/Container";
import { site } from "@/lib/site";

const reasons = [
  {
    title: "Fleksibilitas Pembayaran",
    desc: "Bayar tunai di outlet atau transfer — pembayaran dilakukan setelah cucian ditimbang dan harga final diketahui.",
    icon: <path d="M3 10h18M7 15h2m4 0h4M5 6h14a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2Z" />,
  },
  {
    title: "Riwayat & Pelacakan Digital",
    desc: "Setiap pesanan tercatat digital, dari penjemputan, proses cuci, hingga pengantaran kembali.",
    icon: <path d="M12 8v5l3 3M12 3a9 9 0 1 0 9 9" />,
  },
  {
    title: "Staf Terlatih & Berkomitmen",
    desc: "Setiap outlet ditangani oleh staf yang terlatih menjaga kualitas dan tanggung jawab pengerjaan.",
    icon: <path d="M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM4 21v-1a6 6 0 0 1 6-6h4a6 6 0 0 1 6 6v1" />,
  },
];

export function WhyUs() {
  return (
    <section id="kenapa-kami" className="bg-white py-24">
      <Container className="grid items-center gap-14 lg:grid-cols-2">
        <div>
          <span className="text-sm font-semibold uppercase tracking-wide text-brand-600">
            Kenapa {site.name}
          </span>
          <h2 className="mt-3 font-heading text-3xl font-bold text-navy-900 sm:text-4xl">
            Dipercaya Karena Prosesnya Jelas
          </h2>
          <p className="mt-4 max-w-lg text-slate-600">
            Kami membangun setiap outlet dengan standar operasional yang
            sama, supaya pengalaman laundry Anda konsisten — di mana pun
            outlet yang Anda pilih.
          </p>

          <div className="mt-10 space-y-8">
            {reasons.map((item) => (
              <div key={item.title} className="flex gap-4">
                <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-navy-900 text-white">
                  <svg
                    viewBox="0 0 24 24"
                    className="h-6 w-6"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.7"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    {item.icon}
                  </svg>
                </div>
                <div>
                  <h3 className="font-heading font-semibold text-navy-900">
                    {item.title}
                  </h3>
                  <p className="mt-1 text-sm leading-relaxed text-slate-500">
                    {item.desc}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="relative">
          <div className="aspect-[4/5] w-full rounded-3xl bg-gradient-to-br from-brand-100 via-brand-50 to-white p-8 shadow-inner">
            <div className="grid h-full grid-cols-2 grid-rows-2 gap-4">
              <div className="rounded-2xl bg-white p-5 shadow-sm">
                <p className="text-3xl font-bold text-brand-600">30+</p>
                <p className="mt-1 text-xs text-slate-500">Outlet aktif</p>
              </div>
              <div className="rounded-2xl bg-brand-600 p-5 text-white shadow-sm">
                <p className="text-3xl font-bold">4.9/5</p>
                <p className="mt-1 text-xs text-brand-50">Rata-rata rating</p>
              </div>
              <div className="rounded-2xl bg-navy-900 p-5 text-white shadow-sm">
                <p className="text-3xl font-bold">10rb+</p>
                <p className="mt-1 text-xs text-slate-300">Order selesai</p>
              </div>
              <div className="rounded-2xl bg-white p-5 shadow-sm">
                <p className="text-3xl font-bold text-brand-600">24/7</p>
                <p className="mt-1 text-xs text-slate-500">Pemesanan online</p>
              </div>
            </div>
          </div>
        </div>
      </Container>
    </section>
  );
}
