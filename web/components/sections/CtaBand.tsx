import { Container } from "@/components/ui/Container";
import { LinkButton } from "@/components/ui/Button";

export function CtaBand() {
  return (
    <section className="bg-navy-900 py-16">
      <Container className="flex flex-col items-center justify-between gap-6 text-center sm:flex-row sm:text-left">
        <div>
          <h2 className="font-heading text-2xl font-bold text-white sm:text-3xl">
            Siap laundry tanpa ribet?
          </h2>
          <p className="mt-2 max-w-md text-sm text-slate-300">
            Daftar sekarang dan buat pesanan pertama Anda dalam hitungan
            menit.
          </p>
        </div>
        <div className="flex flex-wrap justify-center gap-4">
          <LinkButton href="/register">Daftar Gratis</LinkButton>
          <LinkButton href="/#outlet" variant="outline">
            Cari Outlet
          </LinkButton>
        </div>
      </Container>
    </section>
  );
}
