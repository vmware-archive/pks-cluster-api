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

	"github.com/gorilla/mux"
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

	r := mux.NewRouter()

	// fake oauth token handler
	r.HandleFunc("/oauth/token", tokenHandler)

	r.HandleFunc("/v1/clusters", clusterCreationHandler(
		gcpProject,
		mustReadTemplate("cluster", clusterTmplPath),
		mustReadTemplate("master", masterTmplPath),
	)).Methods("POST")

	r.HandleFunc("/v1/clusters/{cluster}", clusterDeletionHandler).Methods("DELETE")

	log.Fatal(http.ListenAndServeTLS(":8443", "server.crt", "server.key", r))
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

func clusterCreationHandler(gcpProject string, cluster, master *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := &struct {
			Name string `json:"name"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(body); err != nil {
			log.Printf("Error decoding request body: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		if err := kubectl("create", "ns", body.Name); err != nil {
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
}

func clusterDeletionHandler(w http.ResponseWriter, r *http.Request) {
	cluster, found := mux.Vars(r)["cluster"]
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := kubectl("-n", cluster, "delete", "--all", "machines"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := kubectl("-n", cluster, "delete", "--all", "clusters"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := kubectl("delete", "ns", cluster); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(204)
}

func mustReadTemplate(name, path string) *template.Template {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Error reading %s: %#v", masterTmplPath, err)
	}
	return template.Must(template.New(name).Parse(string(data)))
}

func kubectl(args ...string) error {
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
