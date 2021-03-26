package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var generateKeyCmd GenerateKeyConfig

var GenerateKeyCommand = &cobra.Command{
	Use:   "generate-key",
	Short: "Generate RSA key for use with Concourse components",
	Long:  `TODO`,
	RunE:  ExecuteGenerateKey,
}

func init() {
	GenerateKeyCommand.Flags().StringVarP(&generateKeyCmd.Type, "type", "t", string(RSAKeyType), "The type of key to generate.")
	GenerateKeyCommand.Flags().StringVarP(&generateKeyCmd.FilePath, "filename", "f", "", "File path where the key shall be created. When generating ssh keys, the public key will be stored in a file with the same name but with '.pub' appended.")
	GenerateKeyCommand.Flags().IntVarP(&generateKeyCmd.Bits, "bits", "b", 4096, "The number of bits in the key to create.")

	GenerateKeyCommand.MarkFlagRequired("filename")
}

type GenerateKeyConfig struct {
	Type string

	FilePath string
	Bits     int
}

type ValidKeyTypes string

const (
	RSAKeyType ValidKeyTypes = "rsa"
	SSHKeyType ValidKeyTypes = "ssh"
)

func (g GenerateKeyConfig) Validate() error {
	if g.Type != string(RSAKeyType) || g.Type != string(SSHKeyType) {
		return fmt.Errorf("generate key type %s is not valid. Valid types include %s and %s", g.Type, RSAKeyType, SSHKeyType)
	}

	return nil
}

func ExecuteGenerateKey(cmd *cobra.Command, args []string) error {
	err := generateKeyCmd.Validate()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, generateKeyCmd.Bits)
	if err != nil {
		return fmt.Errorf("failed to generate key: %s", err)
	}

	privateKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	keyFile, err := os.Create(generateKeyCmd.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %s", err)
	}

	err = pem.Encode(keyFile, privateKey)
	if err != nil {
		return fmt.Errorf("failed to write key: %s", err)
	}

	err = keyFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close key file: %s", err)
	}

	fmt.Println("wrote private key to", generateKeyCmd.FilePath)

	if generateKeyCmd.Type == "ssh" {
		pubFilePath := generateKeyCmd.FilePath + ".pub"

		pubKeyFile, err := os.Create(pubFilePath)
		if err != nil {
			return fmt.Errorf("failed to create key file: %s", err)
		}

		sshPubKey, err := ssh.NewPublicKey(key.Public())
		if err != nil {
			return fmt.Errorf("failed to convert ssh public key: %s", err)
		}

		_, err = pubKeyFile.Write(ssh.MarshalAuthorizedKey(sshPubKey))
		if err != nil {
			return fmt.Errorf("failed to write public key: %s", err)
		}

		err = pubKeyFile.Close()
		if err != nil {
			return fmt.Errorf("failed to close key file: %s", err)
		}

		fmt.Println("wrote ssh public key to", pubFilePath)
	}

	return nil
}
