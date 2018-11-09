# PKS Cluster API

**PROOF-OF-CONCEPT**: BOSH-less implementation of PKS API based on [Kubernetes Cluster API](https://github.com/kubernetes-sigs/cluster-api).

### Supported IaaSes
* GCP ([Uses Cluster API Provider for GCP](https://github.com/kubernetes-sigs/cluster-api-provider-gcp))

### Supported Lifecycle Events
* Cluster Creation
* Cluster Deletion
* Cluster Listing
 
### Installation

#### 1. Prepare Control/System Cluster

Clone the GCP provider repo:
```
git clone https://github.com/kubernetes-sigs/cluster-api-provider-gcp.git .
cd cluster-api-provider-gcp/cmd/clusterctl/examples/google
```

Create service accounts in GCP (must be logged in via `gcloud`):
```
./generate-yaml.sh
```

Install necessary CRDs and Controller Manager:
```
cd ../../../..
kustomize build config/default/ > cmd/clusterctl/examples/google/out/provider-components.yaml
echo "---" >> cmd/clusterctl/examples/google/out/provider-components.yaml
kustomize build vendor/sigs.k8s.io/cluster-api/config/default/ >> cmd/clusterctl/examples/google/out/provider-components.yaml
go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
kubectl apply -f config/crds
kustomize build config/default | kubectl apply -f -
cd vendor/sigs.k8s.io/cluster-api/ && kubectl apply -f config/crds
```

#### 2. Deploy PKS API

Update `deployment.yaml` to include your GCP project name.

Deploy PKS API:
```
kubectl apply -f deployment.yaml
```

#### 3. Use the PKS API
Get public IP for the API:
```
PKS_API=$(kubectl get svc pks-api -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

Target PKS CLI:
```
pks login -a https://${PKS_API}:8443 -u none -p none -k
```

Create Cluster:
```
pks create-cluster test -e test.example.com -p none
```

List Clusters:
```
pks clusters
```

Delete Cluster:
```
pks delete-cluster test
```
