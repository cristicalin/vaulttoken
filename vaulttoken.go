package main

import (
	"bytes"
	"encoding/json"
	"flag"
	// "fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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
		RenewToken  bool   `hcl:"renew_token"`
	} `hcl:"vault"`
}

type VaultToken struct {
	Auth struct {
		ClientToken string `json:"client_token"`
	} `json:"auth"`
	Errors []string `json:"errors"`
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
	auth_url := flag.String("auth-url", "/v1/auth/kubernetes/login",
		"The Vault authentication URL")
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

	token := map[string]string{"jwt": string(token_file_content),
		"role": *role}
	json_token, err := json.Marshal(token)
	check(err)

	uri, err := url.Parse(config.Vault.Address)
	check(err)
	uri.Path = *auth_url

	response, err := http.Post(uri.String(), "application/json",
		bytes.NewBuffer(json_token))
	check(err)

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	var vault_token VaultToken
	err = json.Unmarshal(body, &vault_token)
	check(err)

	if len(vault_token.Errors) > 0 {
		log.Fatal("Error trying to authenticate to vault: %v", vault_token.Errors)
	}

	cmd := exec.Command("consul-template",
		append(strings.Fields(*consul_template_params),
			"-config="+*config_file)...)
	cmd.Env = append(os.Environ(),
		"VAULT_TOKEN="+vault_token.Auth.ClientToken)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	check(err)
}
