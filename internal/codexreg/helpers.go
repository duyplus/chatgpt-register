package codexreg

import (
	cryptorand "crypto/rand"
	"math/big"
	"strconv"
)

var firstNames = []string{
	"Alex", "Jamie", "Taylor", "Jordan", "Casey", "Morgan", "Riley", "Avery", "Quinn", "Parker", "Cameron", "Reese", "Skyler", "Drew", "Emerson",
	"Anh", "Bao", "Binh", "Cuong", "Duy", "Giang", "Ha", "Hai", "Hieu", "Hoang", "Hung", "Huy", "Khoa", "Khanh", "Linh", "Long", "Minh", "Nam", "Phong", "Phuc", "Quan", "Quang", "Son", "Thang", "Thanh", "Thao", "Thinh", "Toan", "Trang", "Trung", "Tu", "Tuan", "Viet", "Vinh", "Vu",
}
var lastNames = []string{
	"Ray", "Lee", "Cole", "Reed", "Hunt", "Ford", "Shaw", "Gray", "Vance", "Wolfe", "Brooks", "Hayes", "Pierce", "Quinn", "Sloan",
	"Nguyen", "Tran", "Le", "Pham", "Hoang", "Huynh", "Vu", "Vo", "Dang", "Bui", "Do", "Ho", "Ngo", "Duong", "Ly", "Dinh", "Doan", "Lam", "Trinh", "Mai", "Cao",
}

func ri(n int) int {
	if n <= 0 {
		return 0
	}
	v, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}

// genName Random English full name.
func genName() string {
	return firstNames[ri(len(firstNames))] + " " + lastNames[ri(len(lastNames))]
}

// genAge Random adult age (18-45).
func genAge() string {
	return strconv.Itoa(18 + ri(28))
}

// GenPassword Generates random password satisfying strength requirements (upper/lower/digits and special chars @, $, !).
func GenPassword(n int) string {
	const lower = "abcdefghijkmnpqrstuvwxyz"
	const upper = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const digit = "23456789"
	const special = "@$!"
	all := lower + upper + digit + special
	if n < 12 {
		n = 12
	}
	b := make([]byte, n)
	b[0] = upper[ri(len(upper))]
	b[1] = lower[ri(len(lower))]
	b[2] = digit[ri(len(digit))]
	b[3] = '@'
	b[4] = '$'
	b[5] = '!'
	for i := 6; i < n; i++ {
		b[i] = all[ri(len(all))]
	}
	// Fisher-Yates shuffle
	for i := len(b) - 1; i > 0; i-- {
		j := ri(i + 1)
		b[i], b[j] = b[j], b[i]
	}
	// Guarantee the first character is always an alphabet letter (A-Z, a-z)
	if !((b[0] >= 'a' && b[0] <= 'z') || (b[0] >= 'A' && b[0] <= 'Z')) {
		for i := 1; i < len(b); i++ {
			if (b[i] >= 'a' && b[i] <= 'z') || (b[i] >= 'A' && b[i] <= 'Z') {
				b[0], b[i] = b[i], b[0]
				break
			}
		}
	}
	return string(b)
}
