package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/codegangsta/cli"
	"github.com/concourse/atc"
	"gopkg.in/yaml.v2"
)

func Configure(c *cli.Context) {
	atcURL := c.GlobalString("atcURL")
	insecure := c.GlobalBool("insecure")
	configPath := c.String("config")
	asJSON := c.Bool("json")

	atcRequester := newAtcRequester(atcURL, insecure)

	if configPath == "" {
		getConfig(atcRequester, asJSON)
	} else {
		setConfig(atcRequester, configPath)
	}
}

func getConfig(atcRequester *atcRequester, asJSON bool) {
	getConfig, err := atcRequester.CreateRequest(
		atc.GetConfig,
		nil,
		nil,
	)
	if err != nil {
		log.Fatalln(err)
	}

	resp, err := atcRequester.httpClient.Do(getConfig)
	if err != nil {
		log.Println("failed to get config:", err, resp)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		log.Println("bad response when getting config:", resp.Status)
		os.Exit(1)
	}

	var config atc.Config
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		log.Println("invalid config from server:", err)
		os.Exit(1)
	}

	var payload []byte
	if asJSON {
		payload, err = json.Marshal(config)
	} else {
		payload, err = yaml.Marshal(config)
	}

	if err != nil {
		log.Println("failed to marshal config to YAML:", err)
		os.Exit(1)
	}

	fmt.Printf("%s", payload)
}

func setConfig(atcRequester *atcRequester, configPath string) {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalln(err)
	}

	var config atc.Config
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalln(err)
	}

	payload, err := json.Marshal(config)
	if err != nil {
		log.Fatalln(err)
	}

	setConfig, err := atcRequester.CreateRequest(
		atc.SaveConfig,
		nil,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		log.Fatalln(err)
	}

	setConfig.Header.Set("Content-Type", "application/json")

	resp, err := atcRequester.httpClient.Do(setConfig)
	if err != nil {
		log.Println("failed to save config:", err, resp)
		os.Exit(1)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(os.Stderr, resp.Body)
		os.Exit(1)
	}
}
