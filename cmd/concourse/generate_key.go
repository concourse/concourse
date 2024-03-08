package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"

	bespec "github.com/concourse/concourse/worker/runtime/spec"

	"golang.org/x/crypto/ssh"
)

type GenerateKeyCommand struct {
	Type string `short:"t"  long:"type"  default:"rsa"  choice:"rsa"  choice:"ssh"  description:"The type of key to generate."`

	FilePath string `short:"f"  long:"filename"  required:"true"  description:"File path where the key shall be created. When generating ssh keys, the public key will be stored in a file with the same name but with '.pub' appended."`
	Bits     int    `short:"b"  long:"bits"      default:"4096"   description:"The number of bits in the key to create."`
}

func (cmd *GenerateKeyCommand) Execute(args []string) error {
	key, err := rsa.GenerateKey(rand.Reader, cmd.Bits)
	if err != nil {
		return fmt.Errorf("failed to generate key: %s", err)
	}

	privateKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	keyFile, err := os.Create(cmd.FilePath)
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

	fmt.Println("wrote private key to", cmd.FilePath)

	if cmd.Type == "ssh" {
		pubFilePath := cmd.FilePath + ".pub"

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

type ExtractInternalConfigCommand struct {
	FilePath string `short:"f"  long:"filename"  required:"true"  description:"File path where the key shall be created. When generating ssh keys, the public key will be stored in a file with the same name but with '.pub' appended."`

	Seccomp bool `long:"seccomp" required:"false"  description:"Extract the default builtin seccomp filter"`
}

func (cmd *ExtractInternalConfigCommand) Execute(args []string) error {
	var dest = cmd.FilePath
	if cmd.Seccomp {
		seccompfilter := bespec.GetDefaultSeccompProfile()
		bytes, err := json.Marshal(seccompfilter)
		if err != nil {
			return fmt.Errorf("failed to serialize key file: %s", err)
		}
		err = ioutil.WriteFile(dest, bytes, 0644)
		if err != nil {
			return fmt.Errorf("failed to write json to file: %s @ %s", err, dest)
		}
		return nil
	} else {
		return fmt.Errorf("Nothing to extract, use one of the optional flags for the subcommand")
	}
}
