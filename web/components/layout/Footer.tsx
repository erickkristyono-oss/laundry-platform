import Link from "next/link";
import { Container } from "@/components/ui/Container";
import { Logo } from "@/components/ui/Logo";
import { site } from "@/lib/site";

export function Footer() {
  return (
    <footer className="bg-navy-900 text-slate-300">
      <Container className="grid gap-10 py-14 sm:grid-cols-2 lg:grid-cols-4">
        <div>
          <Logo inverted />
          <p className="mt-4 max-w-xs text-sm leading-relaxed text-slate-400">
            {site.description}
          </p>
        </div>

        <div>
          <h3 className="mb-4 text-sm font-semibold uppercase tracking-wide text-white">
            Layanan
          </h3>
          <ul className="space-y-2.5 text-sm text-slate-400">
            <li>
              <Link href="/#layanan" className="hover:text-white">
                Cuci Kiloan (Wash &amp; Fold)
              </Link>
            </li>
            <li>
              <Link href="/#layanan" className="hover:text-white">
                Cuci Express
              </Link>
            </li>
            <li>
              <Link href="/#layanan" className="hover:text-white">
                Dry Clean
              </Link>
            </li>
            <li>
              <Link href="/#layanan" className="hover:text-white">
                Sepatu &amp; Boneka
              </Link>
            </li>
          </ul>
        </div>

        <div>
          <h3 className="mb-4 text-sm font-semibold uppercase tracking-wide text-white">
            Perusahaan
          </h3>
          <ul className="space-y-2.5 text-sm text-slate-400">
            <li>
              <Link href="/#outlet" className="hover:text-white">
                Jaringan Outlet
              </Link>
            </li>
            <li>
              <Link href="/staff/login" className="hover:text-white">
                Portal Staf
              </Link>
            </li>
            <li>
              <a href={`mailto:${site.supportEmail}`} className="hover:text-white">
                Hubungi Kami
              </a>
            </li>
          </ul>
        </div>

        <div>
          <h3 className="mb-4 text-sm font-semibold uppercase tracking-wide text-white">
            Area Layanan
          </h3>
          <p className="text-sm text-slate-400">
            {site.serviceAreas.join(" · ")} dan sekitarnya.
          </p>
        </div>
      </Container>

      <div className="border-t border-white/10 py-6">
        <Container className="flex flex-col items-center justify-between gap-2 text-xs text-slate-500 sm:flex-row">
          <p>© {new Date().getFullYear()} {site.name}. All rights reserved.</p>
          <p>Dibuat dengan proses laundry yang transparan &amp; terlacak.</p>
        </Container>
      </div>
    </footer>
  );
}
