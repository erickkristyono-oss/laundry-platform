import { Container } from "@/components/ui/Container";
import { WaveDivider } from "@/components/ui/WaveDivider";

const highlights = [
  {
    title: "Jaringan Multi-Outlet",
    desc: "Pilih outlet terdekat, atau biarkan sistem merekomendasikan otomatis berdasarkan lokasi Anda.",
    icon: (
      <path d="M12 2 2 7l10 5 10-5-10-5Zm0 8-8-4v6l8 4 8-4V6l-8 4Zm-8 8 8 4 8-4" />
    ),
  },
  {
    title: "Jemput & Antar",
    desc: "Tidak perlu keluar rumah — tim kami menjemput cucian dan mengantarnya kembali saat selesai.",
    icon: <path d="M3 13h13l4 4h1a2 2 0 0 0 2-2v-3l-3-4h-4V6H3v7Zm3 7a2 2 0 1 0 0-4 2 2 0 0 0 0 4Zm11 0a2 2 0 1 0 0-4 2 2 0 0 0 0 4Z" />,
  },
  {
    title: "Harga Transparan",
    desc: "Berat aktual & harga dikunci sejak proses timbang — tanpa biaya tersembunyi setelah pembayaran.",
    icon: <path d="M12 1v22M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />,
  },
  {
    title: "Status Real-Time",
    desc: "Pantau progres cucian Anda dari diterima, dicuci, hingga siap diambil — kapan saja.",
    icon: <path d="M12 8v4l3 3M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20Z" />,
  },
];

export function TrustBand() {
  return (
    <section className="bg-brand-500 text-white">
      <WaveDivider color="fill-white" flip />

      <Container className="grid gap-10 pb-16 sm:grid-cols-2 lg:grid-cols-4">
        {highlights.map((item) => (
          <div key={item.title} className="text-center sm:text-left">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-white/15 sm:mx-0">
              <svg
                viewBox="0 0 24 24"
                className="h-6 w-6"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                {item.icon}
              </svg>
            </div>
            <h3 className="mt-4 font-heading text-lg font-semibold">
              {item.title}
            </h3>
            <p className="mt-2 text-sm leading-relaxed text-brand-50">
              {item.desc}
            </p>
          </div>
        ))}
      </Container>

      <WaveDivider color="fill-white" />
    </section>
  );
}
