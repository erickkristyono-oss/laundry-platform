// Central place for brand copy so a rebrand touches one file, not every
// component. "LaundryKu" is a placeholder brand name — replace when the
// real business name is decided.
export const site = {
  name: "LaundryKu",
  tagline: "Laundry kiloan multi-outlet, jemput & antar ke rumah Anda",
  description:
    "Platform laundry multi-outlet dengan pickup & delivery, pelacakan status real-time, dan harga transparan.",
  whatsappNumber: process.env.NEXT_PUBLIC_WHATSAPP_NUMBER ?? "6281234567890",
  supportEmail: "halo@laundryku.example",
  serviceAreas: ["Jakarta", "Bekasi", "Depok", "Tangerang", "Bogor"],
};

export const apiBaseUrl =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
