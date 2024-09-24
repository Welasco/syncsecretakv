/*
Copyright 2024 welasco.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azcertificates"
	"github.com/welasco/syncsecretakv/api/api/v1alpha1"
	apiv1alpha1 "github.com/welasco/syncsecretakv/api/api/v1alpha1"

	"crypto/x509"
	"encoding/json"
	"encoding/pem"
)

// SyncSecretAKVReconciler reconciles a SyncSecretAKV object
type SyncSecretAKVReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=syncsecretakvs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=syncsecretakvs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=syncsecretakvs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SyncSecretAKV object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SyncSecretAKVReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info("SyncSecretAKVController - Reconciling SyncSecretAKV: " + req.NamespacedName.Name)
	azKeyVaultCertificateName := req.NamespacedName.Namespace + "-" + req.NamespacedName.Name

	// TODO(user): your logic here

	// Load the Config object from the namespace
	// LoadConfig function is defined in the api package at internal/controller/api/config_controller.go
	config, err := LoadConfig(ctx, r.Client)
	if err != nil {
		log.Log.Error(err, "SyncSecretAKVController - Config not found. Unable to cind a Config in namespace: "+req.NamespacedName.Namespace+". Unable to find ClusterConfig in the cluster.")
		return ctrl.Result{}, err
	}

	syncSecretAKV := &apiv1alpha1.SyncSecretAKV{}
	if err := r.Get(ctx, req.NamespacedName, syncSecretAKV); err != nil && errors.IsNotFound(err) {
		//log.Log.Error(err, "SyncSecretAKVController - Unable to fetch SyncSecretAKV, resource was probably deleted")
		log.Log.Info("SyncSecretAKVController - Unable to fetch SyncSecretAKV, resource was probably deleted. SyncSecretAKV: " + req.NamespacedName.Name + ", Namespace: " + req.NamespacedName.Namespace)
		DeleteAzKeyVaultCertificate(config, azKeyVaultCertificateName)

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil && errors.IsNotFound(err) {
		log.Log.Info("SyncSecretAKVController - Unable to fetch Secret, resource was probably deleted. Secret: " + req.NamespacedName.Name + ", Namespace: " + req.NamespacedName.Namespace)
		log.Log.Info("SyncSecretAKVController - Deleting corresponding SyncSecretAKV: " + syncSecretAKV.Name)
		if err := r.Delete(ctx, syncSecretAKV); err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Unable to delete SyncSecretAKV")
		}
		log.Log.Info("SyncSecretAKVController - Successfully Deleted SyncSecretAKV: " + syncSecretAKV.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Import or Update Azure Key Vault Certificate

	// Need to check the revision of the secret to determine if the certificate needs to be updated
	if syncSecretAKV.Spec.SecretResourceVersion != syncSecretAKV.Spec.SyncSecretAKVResourceVersion {

		log.Log.Info("SyncSecretAKVController - Importing or Updating Azure Key Vault Certificate: " + azKeyVaultCertificateName)
		err = ImportOrUpdateAzKeyVaultCertificate(config, azKeyVaultCertificateName, secret)
		if err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Failed to import or update certificate into Azure Key Vault")

			// Update SyncSecretAKV Status
			syncSecretAKV.Status.SyncStatus = "Failed"
			syncSecretAKV.Status.SyncStatusMessage = "Failed to import or update certificate into Azure Key Vault. Error: " + err.Error()
			if err := r.Status().Update(ctx, syncSecretAKV); err != nil {
				log.Log.Error(err, "SyncSecretAKVController - Failed to update SyncSecretAKV status")
			}
			return ctrl.Result{}, nil
		}

		syncSecretAKV.Spec.SyncSecretAKVResourceVersion = syncSecretAKV.Spec.SecretResourceVersion
		if err := r.Update(ctx, syncSecretAKV); err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Failed to update SyncSecretAKV")
			return ctrl.Result{}, nil
		}

		log.Log.Info("SyncSecretAKVController - Successfuly imported or updated Azure Key Vault Certificate: " + azKeyVaultCertificateName)

		// Update SyncSecretAKV Status
		syncSecretAKV.Status.SyncStatus = "Success"
		syncSecretAKV.Status.SyncStatusMessage = "Successfully imported or updated Azure Key Vault Certificate: " + azKeyVaultCertificateName
		if err := r.Status().Update(ctx, syncSecretAKV); err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Failed to update SyncSecretAKV status")
		}

	} else {
		log.Log.Info("SyncSecretAKVController - Azure Key Vault Certificate is up to date: " + azKeyVaultCertificateName)
	}

	return ctrl.Result{}, nil
}

func ConvertToPkcs8PEM(privKey *string) string {

	bytePrivKey := []byte(*privKey)

	// Decode the PEM block
	block, _ := pem.Decode(bytePrivKey)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		log.Log.Info("SyncSecretAKVController - Failed to decode PEM block containing RSA private key")
	}
	// Parse the RSA private key
	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Log.Error(err, "SyncSecretAKVController - Error parsing RSA private key")
	}
	// Convert the RSA private key to PKCS#8 format
	pkcs8PrivateKey, err := x509.MarshalPKCS8PrivateKey(rsaPrivateKey)
	if err != nil {
		log.Log.Error(err, "SyncSecretAKVController - Error converting to PKCS#8")
	}

	// Create a PEM block with the PKCS#8 private key
	pkcs8PemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8PrivateKey,
	}
	// Encode the PKCS#8 private key to PEM format
	pkcs8PemData := pem.EncodeToMemory(pkcs8PemBlock)
	log.Log.Info("SyncSecretAKVController - RSA private key successfully converted to PKCS#8 format")

	return string(pkcs8PemData)

}

func DeleteAzKeyVaultCertificate(config *apiv1alpha1.Config, azKeyVaultCertificateName string) error {

	log.Log.Info("SyncSecretAKVController - Deleting Azure Key Vault Certificate")

	err := error(nil)
	// Create Azure Credential
	clientCertificate := NewAzKeyVaultClientConfig(config)

	if config.Spec.AllowAzKeyVaultCertificateDeletion {

		//Delete Certificate
		_, err := clientCertificate.DeleteCertificate(context.TODO(), azKeyVaultCertificateName, nil)
		if err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Failed to delete certificate from Azure Key Vault")
		}
		log.Log.Info("SyncSecretAKVController - Successfuly deleted Azure Key Vault Certificate: " + azKeyVaultCertificateName)

		// Sleep for 20 seconds to allow the certificate to be purged
		log.Log.Info("SyncSecretAKVController - Sleeping for 20 seconds to allow the certificate to be purged")
		time.Sleep(20 * time.Second)

		//Purge Certificate
		_, err = clientCertificate.PurgeDeletedCertificate(context.TODO(), azKeyVaultCertificateName, nil)
		if err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Failed to purge certificate from Azure Key Vault")
		}
		log.Log.Info("SyncSecretAKVController - Successfuly purged Azure Key Vault Certificate: " + azKeyVaultCertificateName)
	}

	return err
}

func NewAzKeyVaultClientClusterConfig(clusterConfig *v1alpha1.ClusterConfig) *azcertificates.Client {
	return newAzKeyVaultClient(clusterConfig, nil)
}

func NewAzKeyVaultClientConfig(config *v1alpha1.Config) *azcertificates.Client {
	return newAzKeyVaultClient(nil, config)
}

func newAzKeyVaultClient(clusterConfig *v1alpha1.ClusterConfig, config *v1alpha1.Config) *azcertificates.Client {

	var newConfig *v1alpha1.Config
	var cred azcore.TokenCredential

	if clusterConfig != nil {
		newConfig = ConvertToConfig(clusterConfig)
	} else {
		newConfig = config
	}

	keyVaultUrl := newConfig.Spec.AzKeyVaultURL

	if newConfig.Spec.AzKeyVaultClientSecret != "" && newConfig.Spec.AzKeyVaultClientID != "" && newConfig.Spec.AzKeyVaultTenantID != "" {
		log.Log.Info("SyncSecretAKVController - Using Client Secret for Azure Key Vault Authentication with TenantID: " + newConfig.Spec.AzKeyVaultTenantID + ", ClientID: " + newConfig.Spec.AzKeyVaultClientID)
		var err error
		cred, err = azidentity.NewClientSecretCredential(newConfig.Spec.AzKeyVaultTenantID, newConfig.Spec.AzKeyVaultClientID, newConfig.Spec.AzKeyVaultClientSecret, nil)
		if err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Failed to obtain a NewClientSecretCredential")
		}

	} else if newConfig.Spec.AzKeyVaultClientID != "" {
		log.Log.Info("SyncSecretAKVController - Using Managed Identity for Azure Key Vault Authentication with ClientID: " + newConfig.Spec.AzKeyVaultClientID)
		clientID := azidentity.ClientID(newConfig.Spec.AzKeyVaultClientID)
		msiOPtions := azidentity.ManagedIdentityCredentialOptions{ID: &clientID}
		var err error
		cred, err = azidentity.NewManagedIdentityCredential(&msiOPtions)
		if err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Failed to obtain a NewManagedIdentityCredential")
		}
	} else {
		log.Log.Info("SyncSecretAKVController - Using Default Azure Credential for Azure Key Vault Authentication")
		var err error
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			log.Log.Error(err, "SyncSecretAKVController - Failed to obtain a NewDefaultAzureCredential")
		}
	}

	clientCertificate, err := azcertificates.NewClient(keyVaultUrl, cred, nil)
	if err != nil {
		log.Log.Error(err, "SyncSecretAKVController - Failed to create a client connection to Azure Key Vault")
	}
	return clientCertificate
}

func ConvertToConfig(clusterConfig *v1alpha1.ClusterConfig) *v1alpha1.Config {
	var config v1alpha1.Config

	config.Spec.AzKeyVaultURL = clusterConfig.Spec.AzKeyVaultURL
	config.Spec.AzKeyVaultTenantID = clusterConfig.Spec.AzKeyVaultTenantID
	config.Spec.AzKeyVaultClientID = clusterConfig.Spec.AzKeyVaultClientID
	config.Spec.AzKeyVaultClientSecret = clusterConfig.Spec.AzKeyVaultClientSecret
	config.Spec.FilterMatchingLabels = clusterConfig.Spec.FilterMatchingLabels
	config.Spec.FilterMatchingAnnotations = clusterConfig.Spec.FilterMatchingAnnotations
	config.Spec.AllowAzKeyVaultCertificateDeletion = clusterConfig.Spec.AllowAzKeyVaultCertificateDeletion
	config.Spec.FilterMatchingNamespace = clusterConfig.Spec.FilterMatchingNamespace

	return &config
}

func convertToConfig(data interface{}) (*v1alpha1.Config, error) {
	var config v1alpha1.Config

	// Convert the interface to JSON bytes
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON bytes to ConfigSpec
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func ImportOrUpdateAzKeyVaultCertificate(config *apiv1alpha1.Config, azKeyVaultCertificateName string, secret *corev1.Secret) error {

	log.Log.Info("SyncSecretAKVController - Importing or Updating Azure Key Vault Certificate")

	// Create Azure Credential
	clientCertificate := NewAzKeyVaultClientConfig(config)

	var pubKey string
	var privKey string

	// Need to take care of the ca.crt in case it exist
	for key, value := range secret.Data {
		//log.Log.Info("SyncSecretAKVController - Key: ", key, "\n", "Value: ", string(value))
		log.Log.Info("SyncSecretAKVController - Secret Key: " + key)
		if key == "tls.crt" {
			pubKey = string(value)
		}
		if key == "tls.key" {
			privKey = string(value)
		}
	}

	fullCert := ConvertToPkcs8PEM(&privKey)
	fullCert = pubKey + "\n" + fullCert

	//Import Certificate
	_, err := clientCertificate.ImportCertificate(context.TODO(), azKeyVaultCertificateName, azcertificates.ImportCertificateParameters{Base64EncodedCertificate: &fullCert}, nil)
	if err != nil {
		log.Log.Error(err, "SyncSecretAKVController - Failed to import or update certificate into Azure Key Vault")
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *SyncSecretAKVReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.SyncSecretAKV{}).
		Complete(r)
}
