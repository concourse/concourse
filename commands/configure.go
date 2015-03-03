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
	"github.com/onsi/gomega/gexec"
	"github.com/pkg/term"
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

	var newConfig atc.Config
	err = yaml.Unmarshal(configFile, &newConfig)
	if err != nil {
		log.Fatalln(err)
	}

	payload, err := json.Marshal(newConfig)
	if err != nil {
		log.Fatalln(err)
	}

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

	var existingConfig atc.Config
	err = json.NewDecoder(resp.Body).Decode(&existingConfig)
	if err != nil {
		log.Println("invalid config from server:", err)
		os.Exit(1)
	}

	diff(existingConfig, newConfig)

	setConfig, err := atcRequester.CreateRequest(
		atc.SaveConfig,
		nil,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		log.Fatalln(err)
	}

	setConfig.Header.Set("Content-Type", "application/json")

	resp, err = atcRequester.httpClient.Do(setConfig)
	if err != nil {
		println("failed to update configuration: " + err.Error())
		os.Exit(1)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		println("failed to update configuration.")

		indent := gexec.NewPrefixedWriter("  ", os.Stderr)
		fmt.Fprintf(indent, "response code: %s\n", resp.Status)
		fmt.Fprintf(indent, "response body:\n")

		indentAgain := gexec.NewPrefixedWriter("  ", indent)
		io.Copy(indentAgain, resp.Body)
		os.Exit(1)
	}

	fmt.Println("configuration updated")
}

func diff(existingConfig atc.Config, newConfig atc.Config) {
	indent := gexec.NewPrefixedWriter("  ", os.Stdout)

	groupDiffs := diffIndices(GroupIndex(existingConfig.Groups), GroupIndex(newConfig.Groups))
	if len(groupDiffs) > 0 {
		fmt.Println("groups:")

		for _, diff := range groupDiffs {
			diff.WriteTo(indent, "group")
		}
	}

	resourceDiffs := diffIndices(ResourceIndex(existingConfig.Resources), ResourceIndex(newConfig.Resources))
	if len(resourceDiffs) > 0 {
		fmt.Println("resources:")

		for _, diff := range resourceDiffs {
			diff.WriteTo(indent, "resource")
		}
	}

	jobDiffs := diffIndices(JobIndex(existingConfig.Jobs), JobIndex(newConfig.Jobs))
	if len(jobDiffs) > 0 {
		fmt.Println("jobs:")

		for _, diff := range jobDiffs {
			diff.WriteTo(indent, "job")
		}
	}

	if askToConfirm() {
		println("y")
	} else {
		println("")
		println("bailing out")
		os.Exit(1)
	}
}

func askToConfirm() bool {
	fmt.Printf("apply configuration? (y/n): ")

	var in io.Reader

	term, err := term.Open(os.Stdin.Name())
	if err == nil {
		err = term.SetRaw()
		if err != nil {
			log.Fatalln("failed to set raw:", term)
		}

		defer term.Restore()

		in = term
	} else {
		in = os.Stdin
	}

	ans := make([]byte, 1)
	n, err := in.Read(ans)
	if err != nil {
		log.Fatalln("failed to read response:", err)
	}

	if n == 0 {
		log.Fatalln("no response")
	}

	return ans[0] == 'y'
}
