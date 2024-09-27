---
# Feel free to add content and custom Front Matter to this file.
# To modify the layout, see https://jekyllrb.com/docs/themes/#overriding-theme-defaults

layout: default
---

# SyncSecretAKV Kubernetes Controller

## SyncSecretAKV Kubernetes Controller for Synchronizing TLS Secrets to Azure Key Vault

This Kubernetes Controller is designed to synchronize TLS Secrets created by Cert-Manager to Azure Key Vault. Cert-Manager is another Kubernetes controller that issues certificates from Let's Encrypt. The primary purpose of this controller is to enable the reuse of Let's Encrypt TLS certificates across various Azure resources, such as Azure Application Gateway and Azure VMs.

## Features

- **Automated Synchronization**: Seamlessly syncs TLS Secrets from Kubernetes to Azure Key Vault.
- **Cert-Manager Integration**: Works in conjunction with Cert-Manager to manage certificate issuance and renewal.
- **Azure Resource Compatibility**: Allows Let's Encrypt TLS certificates to be utilized by any Azure resource that supports Azure Key Vault.

## Use Cases

- **Azure Application Gateway**: Secure your web applications with Let's Encrypt certificates stored in Azure Key Vault.
- **Azure VMs**: Easily manage and deploy TLS certificates to your virtual machines.

## Getting Started

1. [**Install Cert-Manager**](#1-install-cert-manager): Ensure Cert-Manager is installed and configured in your Kubernetes cluster.
2. [**Configure Azure Key Vault**](#2-configure-azure-key-vault): Set up your Azure Key Vault and configure the necessary permissions.
3. [**Deploy the Controller**](#3-deploy-the-controller): Deploy this Kubernetes Controller to your cluster.
4. [**Configuring SyncSecretAKV controller**](#4-configuring-syncsecretakv-controller): The controller will automatically synchronize TLS Secrets from Cert-Manager to Azure Key Vault.
5. [**Filtering**](#5-filtering): Filter which TLS Secrets you would like to sync based in Labels and Annotations, or based in the namespace.

## 1. **Install Cert-Manager**

Ensure Cert-Manager is installed and configured in your Kubernetes cluster. You can use the following commands to install Cert-Manager:

```sh
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo update
helm install \
    cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version v1.15.3 \
    --set crds.enabled=true \
    --set enableCertificateOwnerRef=true
```

**_NOTE:_** The command bove is enabling enableCertificateOwnerRef to allow Cert-manager to delete secrets once Ingress rule is removed.

Let's Encrypt supports Staging and Production environment, staging is used for testing environments and these certificates are not valid.

Create a ClusterIssuer resource to setup Cert-manager to issue certificates from Let's Encrypt using staging environment:

**_NOTE:_** Let's Encrypt use ACME protocol to issue the certificate and prove domain ownership. It supports DNS01 or HTTP01 protocols. The example bellow is for HTTP01. For DNS01 using Azure DNS please review official [AzureDNS Cert-manager](https://cert-manager.io/docs/configuration/acme/dns01/azuredns/) documentation.

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: <email>@<domain><.com>
    privateKeySecretRef:
      name: letsencrypt-staging
    solvers:
    - http01:
        ingress:
          class: nginx
          podTemplate:
            spec:
              nodeSelector:
                "kubernetes.io/os": linux
```

Create a ClusterIssuer resource to setup Cert-manager to issue certificates from Let's Encrypt using production environment:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: <email>@<domain><.com>
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
          podTemplate:
            spec:
              nodeSelector:
                "kubernetes.io/os": linux
```

Once the Cert-manager is configured you can create a Ingress rule in your cluster with a special annotation pointing to the ClusterIssuer you would like to use and Cert-manager will automaticaly issue a Let's Encrypt certificate for you.

Here is a sample application using Ingress rule with cert-manager.io/cluster-issuer annotation to issue a certificate from staging in Let's Encrypt. The yaml definition is creating a namespace called vws and a deployment, service and ingress using the special annotation from Cert-manager.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: vws
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vws-app
  namespace: vws
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vws-app
  template:
    metadata:
      labels:
        app: vws-app
    spec:
      containers:
      - name: vwsapp
        image: welasco/nodejsportexhaustion
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        ports:
        - containerPort: 3000
---
apiVersion: v1
kind: Service
metadata:
  name: vws-service
  namespace: vws
spec:
  type: ClusterIP
  ports:
  - port: 3000
  selector:
    app: vws-app
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: vws-ingress-testcertmanager
  namespace: vws
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-staging
    #cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/force-ssl-redirect: "false"
    nginx.ingress.kubernetes.io/ssl-redirect: "false"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - www.vwslab.com
    secretName: vws-secret
  rules:
  - host: www.vwslab.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: vws-service
            port:
              number: 3000
```

After apply the yaml manifest a new TLS secret with Let's Encrypt certificate will be created in the namespace. The secret name will be the name you have defined as your secretName in Ingress rule:

```sh
$ kubectl get secrets -n vws vws-secret

NAME         TYPE                DATA   AGE
vws-secret   kubernetes.io/tls   2      45h
```

Reference:

[Installing Cert-manager with helm](https://cert-manager.io/docs/installation/helm/)

[Deploy cert-manager on Azure Kubernetes Service (AKS) and use Let's Encrypt to sign a certificate for an HTTPS website](https://cert-manager.io/docs/tutorials/getting-started-aks-letsencrypt/)


## 2. **Configure Azure Key Vault**

All the steps will be done using Azure Cli but they can also be accomplished using Azure Portal.

Create a new Azure Key Vault if you don't have one already:

```sh
AKS="<AKS Name>"
KEYVAULT_NAME="<Key Vault Name goes here>"
RG="<Resource Group Name>"
Location="<Location>"
az keyvault create `
    --name $KEYVAULT_NAME `
    --resource-group $RG `
    --location $Location `
    --enable-rbac-authorization
```

Choose your preferable authentication method to allow SyncSecretAKV controller to access Azure Key Vault.



### 2.1 Using Workload Identity or Managed Identity to access Azure Key Vault

If you are going to use Workload Identity or Managed Identity in Azure, you have to create a Managed Identity Credential.

**_NOTE:_** Workload Identity requires additional settings to be configured in AKS cluster. Please check [Deploy and configure workload identity on an Azure Kubernetes Service (AKS) cluster](https://learn.microsoft.com/en-us/azure/aks/workload-identity-deploy-cluster)

The command bellow will create a Managed Identity to be used by SyncSecretAKV controller to access Azure Key Vault:

```sh
USER_ASSIGNED_IDENTITY_NAME="<Managed Identity Name>"
KEYVAULT_NAME="<Key Vault Name goes here>"
RG="<Resource Group Name>"
Location="<Location>"
Subscription="<Subscription ID>"
az identity create `
    --name $USER_ASSIGNED_IDENTITY_NAME `
    --resource-group $RG `
    --location $Location `
    --subscription $Subscription
```

Create a Role assigment to allow the Managed Identity to manage certificates in the Azure Key Vault:

```sh
KEYVAULT_RESOURCE_ID=(az keyvault show --resource-group $RG --name $KEYVAULT_NAME --query id --output tsv)
IDENTITY_PRINCIPAL_ID=(az identity show --name $USER_ASSIGNED_IDENTITY_NAME --resource-group $RG --query principalId --output tsv)
az role assignment create `
    --assignee-object-id $IDENTITY_PRINCIPAL_ID `
    --role "Key Vault Certificates Officer" `
    --scope $KEYVAULT_RESOURCE_ID `
    --assignee-principal-type ServicePrincipal
```

Optionally but recommended, you should assign the same priviledge to your personal account:

```sh
az role assignment create `
    --assignee "<Your Account Here Ex: user@domain.com>" `
    --role "Key Vault Certificates Officer" `
    --scope $KEYVAULT_RESOURCE_ID `
    --assignee-principal-type User
```





### 2.2 Using Service Principal to access Azure Key Vault

If running SyncSecretAKV controller in a On-Premises cluster or ARC enabled Kubernestes cluster you must use Service Principal for authentication.

You can create a new Service Principal using the following command:

```sh
ServicePrincipalName="<Service Principal Name Here Ex: SyncSecretAKV-SP>"
az ad sp create-for-rbac --name $ServicePrincipalName
```

The create Service Principal command you give you an output, save it because it will be necessary to configure SyncSecretAKV controller:

```sh
{
  "appId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "displayName": "app-yak",
  "password": "*************************************",
  "tenant": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

Create a Role assigment to allow the Service Principal to manage certificates in the Azure Key Vault:

```sh
KEYVAULT_RESOURCE_ID=(az keyvault show --resource-group $RG --name $KEYVAULT_NAME --query id --output tsv)
az role assignment create `
    --assignee-object-id "<appId from the previous command>" `
    --role "Key Vault Certificates Officer" `
    --scope $KEYVAULT_RESOURCE_ID `
    --assignee-principal-type ServicePrincipal
```

Optionally but recommended, you should assign the same priviledge to your personal account:

```sh
az role assignment create `
    --assignee "<Your Account Here Ex: user@domain.com>" `
    --role "Key Vault Certificates Officer" `
    --scope $KEYVAULT_RESOURCE_ID `
    --assignee-principal-type User
```



## 3. **Deploy the Controller**

Deploy SyncSecretAKV controller using the commands bellow:

Add a helm repo to your local helm repositories:
```sh
helm repo add syncsecretakv https://welasco.github.io/syncsecretakv
```



### 3.1 Installing SyncSecretAKV controller for Workload Identity

Install SyncSecretAKV controller for Workload Identity authentication to Azure Key Vault.

```sh
IDENTITY_PRINCIPAL_ID=(az identity show --name $USER_ASSIGNED_IDENTITY_NAME --resource-group $RG --query principalId --output tsv)
helm install syncsecretakv syncsecretakv/syncsecretakv \
    --set namespace=syncsecretakv \
    --set workloadIdentity.userAssignedClientId=$IDENTITY_PRINCIPAL_ID
```





### 3.2 Installing SyncSecretAKV controller for Managed Identity or Service Principal

Install SyncSecretAKV controller for Managed Identity or Service Principal authentication:
```sh
helm install syncsecretakv syncsecretakv
```

After the deployment a new namespace and a few resources will be created. To check if everything looks good run the following command:

```sh
$ kubectl get -n syncsecretakv-system Deployment,ServiceAccount,Service

NAME                                               READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/syncsecretakv-controller-manager   1/1     1            1           21h

NAME                                              SECRETS   AGE
serviceaccount/default                            0         21h
serviceaccount/syncsecretakv-controller-manager   0         21h

NAME                                                       TYPE        CLUSTER-IP        EXTERNAL-IP   PORT(S)    AGE
service/syncsecretakv-controller-manager-metrics-service   ClusterIP   xxx.xxx.xxx.xxx   <none>        8443/TCP   21h
```



## 4. **Configuring SyncSecretAKV controller**

SyncSecretAKV controller can be configure to use Workload Identity, Managed Identity or Service Principal to access Azure Key Vault.

It supports Cluster wide configuration or Namespace configuration.


### 4.1 Workload Identity

To setup SyncSecretAKV controller to use Workload Identity you must install the controller using an additional option with userAssignedClientId to allow the creation of the controller pod with the required tags and annotaions for Workload Identity. Please refer to the step "Install SyncSecretAKV controller for Workload Identity authentication to Azure Key Vault."

When using Workload Identity the SyncSecretAKV controller only supports one single Federated Managed Identity to access Azure Key Vault. If you require more than one Azure Key Vault (for instance one Azure Key Vault per namespace) you can only use the managed identity defined during the instalation of the controller to allow access in all Azure Key Vaults.

If you have not yet enable AKS to support Workload Identity you can do it using the following command:

```sh
az aks update \
    --name $AKS \
    --resource-group $RG \
    --enable-oidc-issuer \
    --enable-workload-identity
```

Now associate a federated identity with the managed identity that you created earlier. SyncSecretAKV controller will authenticate to Azure using a short lived Kubernetes ServiceAccount token, and it will be able to impersonate the managed identity that you created in the previous step.

```sh
export SERVICE_ACCOUNT_NAME=syncsecretakv-controller-manager # This is the default Kubernetes ServiceAccount used by the SyncSecretAKV controller.
export SERVICE_ACCOUNT_NAMESPACE=syncsecretakv-system # This is the default namespace for SyncSecretAKV.
export SERVICE_ACCOUNT_ISSUER=$(az aks show --resource-group $RG --name $AKS --query "oidcIssuerProfile.issuerUrl" -o tsv)
az identity federated-credential create \
  --name "cert-manager" \
  --identity-name "${USER_ASSIGNED_IDENTITY_NAME}" \
  --issuer "${SERVICE_ACCOUNT_ISSUER}" \
  --subject "system:serviceaccount:${SERVICE_ACCOUNT_NAMESPACE}:${SERVICE_ACCOUNT_NAME}"
```

To setup SyncSecretAKV controller using Workload Identity for the entire cluster create a ClusterConfig resource with your desired configuration:

```yaml
apiVersion: api.syncsecretakv.io/v1alpha1
kind: ClusterConfig
metadata:
  labels:
    app.kubernetes.io/name: syncsecretakv
    app.kubernetes.io/managed-by: kustomize
  name: clusterconfig-sample
spec:
  azKeyVaultURL: "https://<Azure Key Vault name>.vault.azure.net/"
  allowAzKeyVaultCertificateDeletion: true
  filterMatchingNamespace:
    - "vws"
  # filterMatchingLabels:
  #   label1: "label1"
  #   label2: "label2"
  filterMatchingAnnotations:
    cert-manager.io/issuer-group: "cert-manager.io"
  #   label2: "label2"
```

To setup SyncSecretAKV controller using Workload Identity for a specific namespace create a Config resource with your desired configuration:

```yaml
apiVersion: api.syncsecretakv.io/v1alpha1
kind: Config
metadata:
  labels:
    app.kubernetes.io/name: syncsecretakv
    app.kubernetes.io/managed-by: kustomize
  name: config-sample
  namespace: vws
spec:
  azKeyVaultURL: "https://<Azure Key Vault name>.vault.azure.net/"
  allowAzKeyVaultCertificateDeletion: true
  # filterMatchingLabels:
  #   label1: "label1"
  #   label2: "label2"
  # filterMatchingAnnotations:
  #   label1: "label1"
  #   label2: "label2"
```




### 4.2 Managed Identity

To use Managed Identity you have to associate the Managed Identity with all NodePools (VMSS) of AKS Cluster.

You can use the script bellow to associate your Managed Identity to all VMSS (nodepools) of your cluster:

```sh
managedIdentityResourceId=$(az identity show --name $USER_ASSIGNED_IDENTITY_NAME --resource-group $RG --query id --output tsv)
nodeResourceGroup=$(az aks show -g eslz-spoke --name eslz-aks --query nodeResourceGroup -o tsv)
nodepools=$(az aks nodepool list -g eslz-spoke --cluster-name eslz-aks --query "[].name" --output tsv)
vmssList=$(az vmss list -g $nodeResourceGroup --query "[].name" --output tsv)

for vmss in $vmssList
do
    echo $vmss
    az vmss identity assign -g $nodeResourceGroup -n vmss --identities $managedIdentityResourceId
done
```

You can repeat the previous step for all your managed identities in case you are giving one identity per Azure Key Vault.

To setup SyncSecretAKV controller using Managed Identity for the entire cluster create a ClusterConfig resource with your desired configuration:

```yaml
apiVersion: api.syncsecretakv.io/v1alpha1
kind: ClusterConfig
metadata:
  labels:
    app.kubernetes.io/name: syncsecretakv
    app.kubernetes.io/managed-by: kustomize
  name: clusterconfig-sample
spec:
  azKeyVaultURL: "https://<Azure Key Vault name>.vault.azure.net/"
  azKeyvaultClientId: "<Managed Identity Client ID/appId>"
  allowAzKeyVaultCertificateDeletion: true
  filterMatchingNamespace:
    - "vws"
  # filterMatchingLabels:
  #   label1: "label1"
  #   label2: "label2"
  filterMatchingAnnotations:
    cert-manager.io/issuer-group: "cert-manager.io"
  #   label2: "label2"
```

To setup SyncSecretAKV controller using Managed Identity for a specific namespace create a Config per namespace with your desired configuration:

```yaml
apiVersion: api.syncsecretakv.io/v1alpha1
kind: Config
metadata:
  labels:
    app.kubernetes.io/name: syncsecretakv
    app.kubernetes.io/managed-by: kustomize
  name: config-sample
  namespace: vws
spec:
  azKeyVaultURL: "https://<Azure Key Vault name>.vault.azure.net/"
  allowAzKeyVaultCertificateDeletion: true
  azKeyvaultClientId: "<Managed Identity Client ID/appId>"
  # filterMatchingLabels:
  #   label1: "label1"
  #   label2: "label2"
  # filterMatchingAnnotations:
  #   label1: "label1"
  #   label2: "label2"
```




### 4.3 Service Principal

To use the Service Principal you will need the output after you have created it.

To setup SyncSecretAKV controller using Service Principal for the entire cluster create a ClusterConfig resource with your desired configuration:

```yaml
apiVersion: api.syncsecretakv.io/v1alpha1
kind: ClusterConfig
metadata:
  labels:
    app.kubernetes.io/name: syncsecretakv
    app.kubernetes.io/managed-by: kustomize
  name: clusterconfig-sample
spec:
  azKeyVaultURL: "https://<Azure Key Vault name>.vault.azure.net/"
  azKeyvaultClientId: "<Service Principal appId>"
  azKeyVaultClientSecret: "<Service Principal Secret>"
  azKeyVaultTenantId: "<Microsoft Entra tenant Id>"
  allowAzKeyVaultCertificateDeletion: true
  filterMatchingNamespace:
    - "vws"
  # filterMatchingLabels:
  #   label1: "label1"
  #   label2: "label2"
  filterMatchingAnnotations:
    cert-manager.io/issuer-group: "cert-manager.io"
  #   label2: "label2"
```

To setup SyncSecretAKV controller using Service Principal for a specific namespace create a Config per namespace with your desired configuration:

```yaml
apiVersion: api.syncsecretakv.io/v1alpha1
kind: Config
metadata:
  labels:
    app.kubernetes.io/name: syncsecretakv
    app.kubernetes.io/managed-by: kustomize
  name: config-sample
  namespace: vws
spec:
  azKeyVaultURL: "https://<Azure Key Vault name>.vault.azure.net/"
  allowAzKeyVaultCertificateDeletion: true
  azKeyvaultClientId: "<Service Principal appId>"
  azKeyVaultClientSecret: "<Service Principal Secret>"
  azKeyVaultTenantId: "<Microsoft Entra tenant Id>"
  # filterMatchingLabels:
  #   label1: "label1"
  #   label2: "label2"
  # filterMatchingAnnotations:
  #   label1: "label1"
  #   label2: "label2"
```

## 5. **Filtering**

Oberve that you can filter the controller to watch for specifics screts based in the namespace, labels or annotations by modifing the relative entries filterMatchingNamespace, filterMatchingLabels and filterMatchingAnnotations.

It also allows you to auto delete and purge certificates from Azure Key Vault, by changing allowAzKeyVaultCertificateDeletion to true or false.

Now for all TLS Secrets created by Cert-manager will be synchrnized to Azure Key Vault allowing you to re-use the Let's Encrypt certificate anywhere in Azure.