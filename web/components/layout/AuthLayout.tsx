import Link from "next/link";
import { Logo } from "@/components/ui/Logo";

/**
 * Shared shell for auth pages (customer login/register, staff login): a
 * brand-blue side panel plus a white form card, echoing the landing page's
 * palette so these feel like the same product, not a bolted-on afterthought.
 */
export function AuthLayout({
  title,
  subtitle,
  children,
  footer,
}: {
  title: string;
  subtitle: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
}) {
  return (
    <div className="grid min-h-[calc(100vh-4rem)] lg:grid-cols-2">
      <div className="relative hidden flex-col justify-between overflow-hidden bg-gradient-to-br from-brand-600 to-brand-800 p-12 text-white lg:flex">
        <Link href="/">
          <Logo inverted />
        </Link>

        <div>
          <h2 className="font-heading text-3xl font-bold leading-tight">
            Laundry beres tanpa
            <br />
            keluar rumah.
          </h2>
          <p className="mt-4 max-w-sm text-brand-50">
            Pantau setiap pesanan dari dijemput sampai diantar kembali,
            langsung dari akun Anda.
          </p>
        </div>

        <p className="text-xs text-brand-100">
          © {new Date().getFullYear()} LaundryKu. All rights reserved.
        </p>

        <div className="pointer-events-none absolute -right-24 -top-24 h-72 w-72 rounded-full bg-white/10" />
        <div className="pointer-events-none absolute -bottom-32 -left-16 h-72 w-72 rounded-full bg-white/10" />
      </div>

      <div className="flex items-center justify-center px-6 py-16">
        <div className="w-full max-w-sm">
          <div className="mb-8 lg:hidden">
            <Link href="/">
              <Logo />
            </Link>
          </div>

          <h1 className="font-heading text-2xl font-bold text-navy-900">
            {title}
          </h1>
          <p className="mt-2 text-sm text-slate-500">{subtitle}</p>

          <div className="mt-8">{children}</div>

          {footer && <div className="mt-6 text-sm text-slate-500">{footer}</div>}
        </div>
      </div>
    </div>
  );
}
