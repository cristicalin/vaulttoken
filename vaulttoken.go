package main

import (
	"bytes"
	"encoding/json"
	// "fmt"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/hcl"
)

type Config struct {
	Vault struct {
		Address     string `hcl:"address"`
		Grace       string `hcl:"grace"`
		Token       string `hcl:"token"`
		UnwrapToken bool   `hcl:"unwrap_token"`
		RenewToken  bool   `hcl:"renw_token"`
	} `hcl:"vault"`
}

type VaultToken struct {
	Auth struct {
		ClientToken string `json:"client_token"`
	} `json:"auth"`
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func main() {
	config_file := flag.String("config", "consul-template.conf",
		"The consul-template configuration file")
	token_file := flag.String("token",
		"/var/run/secrets/kubernetes.io/serviceaccount/token",
		"The kubernetes token file")
	consul_template_params := flag.String("params", "",
		"Extra parameters to be passed to the consul-template command")
	role := flag.String("role", "openstack", "The Vault role binding")
	flag.Parse()

	config_file_content, err := ioutil.ReadFile(*config_file)
	check(err)

	token_file_content, err := ioutil.ReadFile(*token_file)
	check(err)

	var config Config
	err = hcl.Decode(&config, string(config_file_content))
	check(err)

	token := map[string]string{"jwt": string(token_file_content), "role": *role}
	json_token, err := json.Marshal(token)
	check(err)

	response, err := http.Post(config.Vault.Address, "application/json",
		bytes.NewBuffer(json_token))
	check(err)

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	var vault_token VaultToken
	err = json.Unmarshal(body, &vault_token)
	check(err)

	var out bytes.Buffer
	cmd := exec.Command("consul-template")
	cmd.Env = append(os.Environ(), "VAULT_TOKEN="+vault_token.Auth.ClientToken)
	cmd.Args = append(strings.Fields(*consul_template_params),
		"-config=", *config_file)
	cmd.Stdout = &out
	err = cmd.Run()
	check(err)

	log.Print(out.String())
}
