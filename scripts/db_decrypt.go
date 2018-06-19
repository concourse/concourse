package main

import (
	"fmt"

	"github.com/concourse/atc/db/encryption"
	"github.com/concourse/flag"
	flags "github.com/jessevdk/go-flags"
)

type ConcryptCommand struct {
	Key        flag.Cipher `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	Ciphertext string      `long:"ciphertext"         description:"the ciphertext to decrypt."`
	Nonce      string      `long:"nonce"              description:"Nonce for decryption."`
}

func main() {
	var command ConcryptCommand
	_, err := flags.Parse(&command)
	if err != nil {
		panic(err)
	}
	plaintext, err := encryption.NewKey(command.Key.AEAD).Decrypt(command.Ciphertext, &command.Nonce)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", plaintext)
}
