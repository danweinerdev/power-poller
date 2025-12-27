// Package protocol implements the TP-Link KASA device communication protocol.
package protocol

// InitialKey is the starting key for the XOR cipher.
const InitialKey byte = 0xAB

// Encrypt encrypts plaintext using TP-Link's XOR cipher with rolling key.
// Each plaintext byte is XOR'd with the current key, and the resulting
// ciphertext byte becomes the key for the next byte.
func Encrypt(plaintext []byte) []byte {
	if len(plaintext) == 0 {
		return []byte{}
	}

	ciphertext := make([]byte, len(plaintext))
	key := InitialKey

	for i, b := range plaintext {
		ciphertext[i] = b ^ key
		key = ciphertext[i] // Rolling key: result becomes next key
	}

	return ciphertext
}

// Decrypt decrypts ciphertext using TP-Link's XOR cipher with rolling key.
// Each ciphertext byte is XOR'd with the current key to recover the plaintext,
// then the ciphertext byte becomes the key for the next byte.
func Decrypt(ciphertext []byte) []byte {
	if len(ciphertext) == 0 {
		return []byte{}
	}

	plaintext := make([]byte, len(ciphertext))
	key := InitialKey

	for i, b := range ciphertext {
		plaintext[i] = b ^ key
		key = b // Rolling key: ciphertext byte becomes next key
	}

	return plaintext
}
