package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"text/template"
)

const (
	gcpProject          = "gcp-project-name"
	kubeletVersion      = "1.9.4"
	controlPlaneVersion = "1.9.4"

	clusterTmplPath = "cluster.yaml.tmpl"
	masterTmplPath  = "master.yaml.tmpl"
)

var (
	clusterTmpl *template.Template
	masterTmpl  *template.Template
)

func main() {
	tmplString, err := ioutil.ReadFile(clusterTmplPath)
	if err != nil {
		log.Fatalf("Could not read %s: %#v", clusterTmplPath, err)
	}
	clusterTmpl = template.Must(template.New("cluster").Parse(string(tmplString)))

	tmplString, err = ioutil.ReadFile(masterTmplPath)
	if err != nil {
		log.Fatalf("Could not read %s: %#v", masterTmplPath, err)
	}
	masterTmpl = template.Must(template.New("master").Parse(string(tmplString)))

	http.HandleFunc("/oauth/token", tokenHandler)
	http.HandleFunc("/v1/clusters", clusterHandler)
	log.Fatal(http.ListenAndServeTLS(":8443", "server.crt", "server.key", nil))
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Token Request")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
		"access_token" : "someaccesstoken",
		"token_type" : "bearer",
		"id_token" : "someidtoken",
		"refresh_token" : "somerefreshtoken",
		"expires_in" : 43199,
		"scope" : "openid oauth.approvals",
		"jti" : "somejti"
	}`))
}

func clusterHandler(w http.ResponseWriter, r *http.Request) {
	body := &struct {
		Name string `json:"name"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(body); err != nil {
		fmt.Printf("Error decoding request body: %#v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tmplData := struct {
		ClusterName         string
		KubeletVersion      string
		ControlPlaneVersion string
		GcpProject          string
	}{
		ClusterName:         body.Name,
		KubeletVersion:      kubeletVersion,
		ControlPlaneVersion: controlPlaneVersion,
		GcpProject:          gcpProject,
	}

	cmd := exec.Command("kubectl", "create", "ns", body.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := kubeApplyTemplate(clusterTmpl, tmplData, os.Stdout); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := kubeApplyTemplate(masterTmpl, tmplData, os.Stdout); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(202)
	w.Write([]byte(fmt.Sprintf(`{
		"kubernetes_master_ips": [
			"string"
		],
		"last_action": "string",
		"last_action_description": "string",
		"last_action_state": "in progress",
		"name": "%s",
		"network_profile_name": "string",
		"parameters": {
			"authorization_mode": "string",
			"kubernetes_master_host": "string",
			"kubernetes_master_port": 8443,
			"kubernetes_worker_instances": 0,
			"nsxt_network_profile": "string",
			"worker_haproxy_ip_addresses": "string"
		},
		"plan_name": "string",
		"uuid": "string"
	}`, body.Name)))
}

func kubeApplyTemplate(t *template.Template, tData interface{}, out io.Writer) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	// cmd := exec.Command("cat")
	cmd.Stdout = out
	cmd.Stderr = out

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not get stdin pipe: %#v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot run kubectl: %#v", err)
	}

	if err := t.Execute(stdinPipe, tData); err != nil {
		return fmt.Errorf("cannot execute cluster template: %#v", err)
	}
	stdinPipe.Close()

	return cmd.Wait()
}
