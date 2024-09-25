# Installing Go Lang
curl -L -o go1.23.1.linux-amd64.tar.gz https://go.dev/dl/go1.23.1.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.1.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo export PATH=$PATH:/usr/local/go/bin >> ~/.bashrc
go version
rm go1.23.1.linux-amd64.tar.gz

# Install Kubebuilder
# download kubebuilder and install locally.
curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/

# Create go Project
go mod init github.com/welasco/syncsecretakv

# Create Kubebuilder project
kubebuilder init --owner "welasco" --domain "syncsecretakv.io"
# Since this project is watching for changes in a secret and keep tracking using a custom resource it was required to enable multigrou support
kubebuilder edit --multigroup=true

# Create API
kubebuilder create api --group core --version v1 --kind Secret
# OBS We don't want to create a resource when prompted because a Secret is already part of Kubernetes we only need a controller

# Create Custom API for a Custom Resource
kubebuilder create api --group api --version v1alpha1 --kind SyncSecretAKV

# Create Custom API for a Custom Resource
kubebuilder create api --group api --version v1alpha1 --kind Config

# Create Custom API Cluster-scoped for Global Configuration
kubebuilder create api --group api --version v1alpha1 --kind ClusterConfig --namespaced=false

# Kubebuilder has the ability to auto generate a CRD and all definitions for the API
# You have to install them in the cluster
make manifests
make install

# Once everything is ready to be published run the docker build
#IMG=welasco/syncsecretakv make docker-build
# Build Docker Images
make docker-build docker-push IMG=welasco/controller-syncsecretakv:latest

# Deploy the controller
make deploy IMG=welasco/controller-syncsecretakv:latest

# Uninstall the controller
make undeploy

# Generate full deployment file
kustomize build config/default > deploy-controller.yaml

#######################
# Creating Helm Chart
#######################
# Create Helm Chart
mkdir charts
helm create charts/syncsecretakv

# Install Helm Chart
cd charts
helm install syncsecretakv syncsecretakv
helm install syncsecretakv syncsecretakv --values syncsecretakv/values.yaml

# Add repo and install it
helm repo add syncsecretakv https://welasco.github.io/syncsecretakv
helm install syncsecretakv syncsecretakv
helm install syncsecretakv syncsecretakv/syncsecretakv \
    --set namespace=syncsecretakv \
    --set workloadIdentity.userAssignedClientId=xxxxx-xxxx-xxxx-xxxx-xxxxx

helm install syncsecretakv syncsecretakv/syncsecretakv --set namespace=syncsecretakv-system --set workloadIdentity.workloadIdentity=xxxxx-xxxx-xxxx-xxxx-xxxxx

# Uninstall Helm Chart
helm uninstall syncsecretakv