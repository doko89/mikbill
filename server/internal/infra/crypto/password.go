package crypto

import "golang.org/x/crypto/bcrypt"

// BcryptCost sesuai tech_spec bagian 10.
const BcryptCost = 12

// HashPassword membuat hash bcrypt dari password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword memverifikasi password terhadap hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
