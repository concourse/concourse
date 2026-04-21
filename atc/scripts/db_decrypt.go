package main

import (
	"fmt"

	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/flag"
	flags "github.com/jessevdk/go-flags"
)

type ConcryptCommand struct {
	Key        flag.Cipher       `long:"encryption-key"            description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	KeyBase64  flag.CipherBase64 `long:"encryption-key-base64"     description:"A base64-encoded 16 or 32 byte key used to encrypt sensitive information before storing it in the database."`
	KeyHex     flag.CipherHex    `long:"encryption-key-hex"        description:"A hex-encoded 16 or 32 byte key used to encrypt sensitive information before storing it in the database."`
	Ciphertext string            `long:"ciphertext"                description:"the ciphertext to decrypt."`
	Nonce      string            `long:"nonce"                     description:"Nonce for decryption."`
}

func main() {
	var command ConcryptCommand
	_, err := flags.Parse(&command)
	if err != nil {
		panic(err)
	}

	key, err := encryption.ResolveKey(command.Key.AEAD, command.KeyBase64.AEAD, command.KeyHex.AEAD)
	if err != nil {
		panic(err)
	}
	if key == nil {
		panic("one of --encryption-key, --encryption-key-base64, or --encryption-key-hex must be provided")
	}

	plaintext, err := key.Decrypt(command.Ciphertext, &command.Nonce)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", plaintext)
}
