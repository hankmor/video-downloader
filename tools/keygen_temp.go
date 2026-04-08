package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

func main() {
	pub, priv, _ := ed25519.GenerateKey(nil)
	fmt.Printf("Private Key: %s\n", base64.StdEncoding.EncodeToString(priv))
	fmt.Printf("Public Key: %s\n", base64.StdEncoding.EncodeToString(pub))
}
