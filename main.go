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
	kubeletVersion      = "1.9.4"
	controlPlaneVersion = "1.9.4"

	clusterTmplPath = "cluster.yaml.tmpl"
	masterTmplPath  = "master.yaml.tmpl"
)

func main() {
	gcpProject := os.Getenv("GCP_PROJECT")
	if gcpProject == "" {
		log.Fatal("GCP_PROJECT is not set")
	}

	// fake oauth token handler
	http.HandleFunc("/oauth/token", tokenHandler)

	// cluster handler
	http.HandleFunc("/v1/clusters", clusterHandler(
		gcpProject,
		mustReadTemplate("cluster", clusterTmplPath),
		mustReadTemplate("master", masterTmplPath),
	))

	log.Fatal(http.ListenAndServeTLS(":8443", "server.crt", "server.key", nil))
}

func mustReadTemplate(name, path string) *template.Template {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Error reading %s: %#v", masterTmplPath, err)
	}
	return template.Must(template.New(name).Parse(string(data)))
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
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

func clusterHandler(gcpProject string, cluster, master *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Serve the resource.
		case http.MethodPost:
			createCluster(w, r, gcpProject, cluster, master)
		case http.MethodPut:
			// Update an existing record.
		case http.MethodDelete:
			// Remove the record.
		default:
			// Give an error message.
		}
	}
}

func createCluster(w http.ResponseWriter, r *http.Request, gcpProject string, cluster, master *template.Template) {
	body := &struct {
		Name string `json:"name"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(body); err != nil {
		log.Printf("Error decoding request body: %#v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	cmd := exec.Command("kubectl", "create", "ns", body.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tmplData := struct {
		ClusterName         string
		KubeletVersion      string
		ControlPlaneVersion string
		GCPProject          string
	}{
		ClusterName:         body.Name,
		KubeletVersion:      kubeletVersion,
		ControlPlaneVersion: controlPlaneVersion,
		GCPProject:          gcpProject,
	}

	if err := kubeApplyTemplate(cluster, tmplData, os.Stdout, os.Stderr); err != nil {
		log.Printf("Error applying cluster template: %#v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := kubeApplyTemplate(master, tmplData, os.Stdout, os.Stderr); err != nil {
		log.Printf("Error applying master template: %#v", err)
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

func kubeApplyTemplate(t *template.Template, tData interface{}, stdout io.Writer, errout io.Writer) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdout = stdout
	cmd.Stderr = errout

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
