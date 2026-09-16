package handler

import "fmt"

// statusLabel mirrors web/components/order/StatusBadge.tsx's statusLabel map
// so the WhatsApp copy matches what the customer sees on the tracking page.
var statusLabel = map[string]string{
	"CREATED":   "Dibuat",
	"RECEIVED":  "Diterima outlet",
	"WEIGHING":  "Sedang ditimbang",
	"WASHING":   "Sedang dicuci",
	"DRYING":    "Sedang dikeringkan",
	"IRONING":   "Sedang disetrika",
	"PACKING":   "Sedang dikemas",
	"READY":     "Siap diambil",
	"PICKED_UP": "Sudah diambil",
	"DELIVERED": "Sudah diantar",
	"COMPLETED": "Selesai",
	"ON_HOLD":   "Ditahan sementara",
	"CANCELLED": "Dibatalkan",
}

func formatIDR(amount int64) string {
	// Simple thousands-separated Rupiah formatting — no locale package needed
	// for a "Rp 22.400"-shaped string.
	s := fmt.Sprintf("%d", amount)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	return "Rp " + string(out)
}

func orderCreatedMessage(customerName, orderCode string) string {
	return fmt.Sprintf(
		"Halo %s, pesanan %s Anda sudah kami terima. Kami akan kabari setiap kali statusnya berubah. Terima kasih sudah memakai LaundryKu!",
		customerName, orderCode,
	)
}

func orderStatusChangedMessage(customerName, orderCode, toStatus string) string {
	label, ok := statusLabel[toStatus]
	if !ok {
		label = toStatus
	}
	return fmt.Sprintf("Halo %s, status pesanan %s Anda sekarang: %s.", customerName, orderCode, label)
}

func paymentPaidMessage(customerName, orderCode string, amount int64) string {
	return fmt.Sprintf(
		"Halo %s, pembayaran pesanan %s sebesar %s sudah kami terima. Terima kasih!",
		customerName, orderCode, formatIDR(amount),
	)
}

func paymentFailedMessage(customerName, orderCode string) string {
	return fmt.Sprintf(
		"Halo %s, pembayaran online untuk pesanan %s gagal diproses. Silakan coba lagi atau bayar tunai di outlet.",
		customerName, orderCode,
	)
}

func paymentRefundedMessage(customerName, orderCode string, amount int64) string {
	return fmt.Sprintf(
		"Halo %s, dana sebesar %s untuk pesanan %s telah kami kembalikan (refund).",
		customerName, orderCode, formatIDR(amount),
	)
}
