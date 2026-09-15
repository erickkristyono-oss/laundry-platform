// Package bizid generates the human-readable business identifiers
// specified in docs/06-database-schema.md §0 (e.g. CUS-xxxxxx,
// ORD-YYYYMMDD-xxxxxx). This is ID-format plumbing only, not a business
// rule, so it belongs in shared/ per docs/03-system-architecture.md §4.
package bizid

import (
	"crypto/rand"
	"math/big"
	"time"
)

const alphabet = "0123456789"

// suffix returns a random n-digit numeric string.
func suffix(n int) string {
	out := make([]byte, n)
	for i := range out {
		digit, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			// crypto/rand failure is unrecoverable for ID generation;
			// fall back to a time-based digit rather than panicking.
			out[i] = alphabet[time.Now().UnixNano()%int64(len(alphabet))]
			continue
		}
		out[i] = alphabet[digit.Int64()]
	}
	return string(out)
}

// Customer returns CUS-xxxxxx.
func Customer() string { return "CUS-" + suffix(6) }

// Order returns ORD-YYYYMMDD-xxxxxx.
func Order() string { return "ORD-" + time.Now().Format("20060102") + "-" + suffix(6) }

// Payment returns PAY-YYYYMMDD-xxxxxx.
func Payment() string { return "PAY-" + time.Now().Format("20060102") + "-" + suffix(6) }

// Refund returns RFD-YYYYMMDD-xxxxxx.
func Refund() string { return "RFD-" + time.Now().Format("20060102") + "-" + suffix(6) }
