package mockdevice

const xorKey byte = 0xAB

// encrypt encrypts plaintext using TP-Link XOR protocol.
func encrypt(plaintext []byte) []byte {
	if len(plaintext) == 0 {
		return []byte{}
	}

	result := make([]byte, len(plaintext))
	key := xorKey

	for i, b := range plaintext {
		result[i] = b ^ key
		key = result[i]
	}

	return result
}

// decrypt decrypts ciphertext using TP-Link XOR protocol.
func decrypt(ciphertext []byte) []byte {
	if len(ciphertext) == 0 {
		return []byte{}
	}

	result := make([]byte, len(ciphertext))
	key := xorKey

	for i, b := range ciphertext {
		result[i] = b ^ key
		key = b
	}

	return result
}
