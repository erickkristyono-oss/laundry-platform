import { Container } from "@/components/ui/Container";

const steps = [
  {
    step: "01",
    title: "Pesan atau Jadwalkan Jemput",
    desc: "Buat pesanan lewat web, datang ke outlet, atau chat WhatsApp — pilih antar sendiri atau dijemput.",
  },
  {
    step: "02",
    title: "Ditimbang & Diberi Harga",
    desc: "Staf menimbang cucian Anda dan harga final dikunci saat itu juga — tanpa biaya kejutan setelahnya.",
  },
  {
    step: "03",
    title: "Dicuci Bertahap",
    desc: "Proses cuci, kering, hingga setrika — status setiap tahap bisa Anda pantau secara real-time.",
  },
  {
    step: "04",
    title: "Siap, Ambil atau Diantar",
    desc: "Setelah rapi dan dikemas, ambil sendiri di outlet atau biarkan kami mengantarnya ke rumah Anda.",
  },
];

export function HowItWorks() {
  return (
    <section id="cara-kerja" className="bg-slate-50 py-24">
      <Container>
        <div className="mx-auto max-w-2xl text-center">
          <span className="text-sm font-semibold uppercase tracking-wide text-brand-600">
            Cara Kerja
          </span>
          <h2 className="mt-3 font-heading text-3xl font-bold text-navy-900 sm:text-4xl">
            Empat Langkah Sederhana
          </h2>
        </div>

        <div className="mt-14 grid gap-8 md:grid-cols-4">
          {steps.map((item, i) => (
            <div key={item.step} className="relative">
              <div className="flex items-center gap-3">
                <span className="font-heading text-3xl font-bold text-brand-200">
                  {item.step}
                </span>
                {i < steps.length - 1 && (
                  <span className="hidden h-px flex-1 bg-slate-200 md:block" />
                )}
              </div>
              <h3 className="mt-4 font-heading text-lg font-semibold text-navy-900">
                {item.title}
              </h3>
              <p className="mt-2 text-sm leading-relaxed text-slate-500">
                {item.desc}
              </p>
            </div>
          ))}
        </div>
      </Container>
    </section>
  );
}
