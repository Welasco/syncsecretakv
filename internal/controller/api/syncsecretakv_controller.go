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
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azcertificates"
	apiv1alpha1 "github.com/welasco/syncsecretakv/api/api/v1alpha1"

	"crypto/x509"
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

	log.Log.Info("Reconciling SyncSecretAKV: " + req.NamespacedName.Name)
	azKeyVaultCertificateName := req.NamespacedName.Namespace + "-" + req.NamespacedName.Name

	// TODO(user): your logic here

	// Load the Config object from the namespace
	// LoadConfig function is defined in the api package at internal/controller/api/config_controller.go
	config, err := LoadConfig(ctx, r.Client)
	if err != nil {
		log.Log.Error(err, "Config not found. Unable to load Config resource from namespace: "+req.NamespacedName.Namespace)
		return ctrl.Result{}, err
	}

	syncSecretAKV := &apiv1alpha1.SyncSecretAKV{}
	if err := r.Get(ctx, req.NamespacedName, syncSecretAKV); err != nil && errors.IsNotFound(err) {
		//log.Log.Error(err, "Unable to fetch SyncSecretAKV, resource was probably deleted")
		log.Log.Info("Unable to fetch SyncSecretAKV, resource was probably deleted. SyncSecretAKV: " + req.NamespacedName.Name + ", Namespace: " + req.NamespacedName.Namespace)
		DeleteAzKeyVaultCertificate(config, azKeyVaultCertificateName)

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil && errors.IsNotFound(err) {
		log.Log.Error(err, "Unable to fetch Secret, resource was probably deleted")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Import or Update Azure Key Vault Certificate
	log.Log.Info("Importing or Updating Azure Key Vault Certificate: " + azKeyVaultCertificateName)
	err = ImportOrUpdateAzKeyVaultCertificate(config, azKeyVaultCertificateName, secret)
	if err != nil {
		log.Log.Error(err, "Failed to import or update certificate into Azure Key Vault")

		// Update SyncSecretAKV Status
		syncSecretAKV.Status.SyncStatus = "Failed"
		syncSecretAKV.Status.SyncStatusMessage = "Failed to import or update certificate into Azure Key Vault"
		if err := r.Status().Update(ctx, syncSecretAKV); err != nil {
			log.Log.Error(err, "Failed to update SyncSecretAKV status")
		}
		return ctrl.Result{}, nil
	}
	log.Log.Info("Successfuly imported or updated Azure Key Vault Certificate: " + azKeyVaultCertificateName)

	// Update SyncSecretAKV Status
	syncSecretAKV.Status.SyncStatus = "Success"
	syncSecretAKV.Status.SyncStatusMessage = "Successfully imported or updated Azure Key Vault Certificate: " + azKeyVaultCertificateName
	if err := r.Status().Update(ctx, syncSecretAKV); err != nil {
		log.Log.Error(err, "Failed to update SyncSecretAKV status")
	}

	return ctrl.Result{}, nil
}

func ConvertToPkcs8PEM(privKey *string) string {

	bytePrivKey := []byte(*privKey)

	// Decode the PEM block
	block, _ := pem.Decode(bytePrivKey)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		log.Log.Info("Failed to decode PEM block containing RSA private key")
	}
	// Parse the RSA private key
	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Log.Error(err, "Error parsing RSA private key")
	}
	// Convert the RSA private key to PKCS#8 format
	pkcs8PrivateKey, err := x509.MarshalPKCS8PrivateKey(rsaPrivateKey)
	if err != nil {
		log.Log.Error(err, "Error converting to PKCS#8")
	}

	// Create a PEM block with the PKCS#8 private key
	pkcs8PemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8PrivateKey,
	}
	// Encode the PKCS#8 private key to PEM format
	pkcs8PemData := pem.EncodeToMemory(pkcs8PemBlock)
	log.Log.Info("RSA private key successfully converted to PKCS#8 format")

	return string(pkcs8PemData)

}

func DeleteAzKeyVaultCertificate(config *apiv1alpha1.Config, azKeyVaultCertificateName string) {

	log.Log.Info("Deleting Azure Key Vault Certificate")

	// Create Azure Credential
	clientCertificate := NewAzKeyVaultClient(config)

	//Delete Certificate
	_, err := clientCertificate.DeleteCertificate(context.TODO(), azKeyVaultCertificateName, nil)
	if err != nil {
		log.Log.Error(err, "Failed to delete certificate from Azure Key Vault")
	}
	log.Log.Info("Successfuly deleted Azure Key Vault Certificate: " + azKeyVaultCertificateName)
}

func NewAzKeyVaultClient(config *apiv1alpha1.Config) *azcertificates.Client {

	keyVaultUrl := config.Spec.AzKeyVaultURL

	if config.Spec.AzKeyVaultTenantID != "" {
		// Set the AZURE_TENANT_ID environment variable
		os.Setenv("AZURE_TENANT_ID", config.Spec.AzKeyVaultTenantID)
	} else {
		_, exists := os.LookupEnv("AZURE_TENANT_ID")
		if exists {
			os.Unsetenv("AZURE_TENANT_ID")
		}
	}

	if config.Spec.AzKeyVaultClientID != "" {
		// Set the AZURE_CLIENT_ID environment variable
		os.Setenv("AZURE_CLIENT_ID", config.Spec.AzKeyVaultClientID)
	} else {
		_, exists := os.LookupEnv("AZURE_CLIENT_ID")
		if exists {
			os.Unsetenv("AZURE_CLIENT_ID")
		}
	}

	if config.Spec.AzKeyVaultClientSecret != "" {
		// Set the AZURE_CLIENT_SECRET environment variable
		os.Setenv("AZURE_CLIENT_SECRET", config.Spec.AzKeyVaultClientSecret)
	} else {
		_, exists := os.LookupEnv("AZURE_CLIENT_SECRET")
		if exists {
			os.Unsetenv("AZURE_CLIENT_SECRET")
		}
	}

	// Need to implement a logic to set the required environment variables for the Azure SDK DefaultCredential

	// Create Azure Credential
	// Implement support for Workload Identity, Managed Identity, Service Principal, and Client Secret
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Log.Error(err, "Failed to obtain a credential")
	}

	//Establish a connection to the Key Vault clientSecret
	clientCertificate, err := azcertificates.NewClient(keyVaultUrl, cred, nil)
	if err != nil {
		log.Log.Error(err, "Failed to create a client connection to Azure Key Vault")
	}
	return clientCertificate
}

func ImportOrUpdateAzKeyVaultCertificate(config *apiv1alpha1.Config, azKeyVaultCertificateName string, secret *corev1.Secret) error {

	log.Log.Info("Importing or Updating Azure Key Vault Certificate")

	// Create Azure Credential
	clientCertificate := NewAzKeyVaultClient(config)

	var pubKey string
	var privKey string

	for key, value := range secret.Data {
		log.Log.Info("Key: ", key, "\n", "Value: ", string(value))
		if key == ".tls.crt" {
			pubKey = string(value)
		}
		if key == ".tls.key" {
			privKey = string(value)
		}
	}

	fullCert := ConvertToPkcs8PEM(&privKey)
	fullCert = pubKey + "\n" + fullCert

	//Import Certificate
	_, err := clientCertificate.ImportCertificate(context.TODO(), fullCert, azcertificates.ImportCertificateParameters{Base64EncodedCertificate: &azKeyVaultCertificateName}, nil)
	// if err != nil {
	// 	log.Log.Error(err, "Failed to import or update certificate into Azure Key Vault")
	// }
	// log.Log.Info("Successfuly imported or updated Azure Key Vault Certificate: " + azKeyVaultCertificateName)
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *SyncSecretAKVReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.SyncSecretAKV{}).
		Complete(r)
}
