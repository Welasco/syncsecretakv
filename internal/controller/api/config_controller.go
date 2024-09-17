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
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/welasco/syncsecretakv/api/api/v1alpha1"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=configs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=api.syncsecretakv.io,resources=configs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Config object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	log.Log.Info("ConfigController - Reconciling Namespace Config: " + req.NamespacedName.Name)

	// Load the Config object from the namespace
	config := &apiv1alpha1.Config{}
	if err := r.Get(ctx, req.NamespacedName, config); err != nil {
		log.Log.Info("ConfigController - Unable to load Config object, the Config object was probably deleted")
		return ctrl.Result{}, err
	}

	// Test if the config is valid by accessing the Azure Key Vault
	// NewAzKeyVaultClient function is defined in the api package at internal/controller/api/syncsecretakv_controller.go
	clientCertificate := NewAzKeyVaultClientConfig(config)

	// List all certificates in the Azure Key Vault to test Config
	pager := clientCertificate.NewListCertificatesPager(nil)

	log.Log.Info("ConfigController - Testing Config by listing certificates in the Azure Key Vault: ")
	for pager.More() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			log.Log.Error(err, "ConfigController - Unable to list certificates in the Azure Key Vault, invalid Config settings")
			config.Status.ConfigStatus = "Failed"
			config.Status.ConfigStatusMessage = "Unable to list certificates in the Azure Key Vault, invalid Config settings. Error: " + err.Error()
			if err := r.Status().Update(ctx, config); err != nil {
				log.Log.Error(err, "ConfigController - Failed to update Config status")
			}
			return ctrl.Result{}, err
		}
		for _, cert := range page.Value {
			log.Log.Info("ConfigController - Certificate Found in Azure Key Vault: " + cert.ID.Name())
		}
	}

	config.Status.ConfigStatus = "Success"
	config.Status.ConfigStatusMessage = "Successfully listed certificates in the Azure Key Vault"
	if err := r.Status().Update(ctx, config); err != nil {
		log.Log.Error(err, "ConfigController - Failed to update Config status")
	}

	return ctrl.Result{}, nil
}

func LoadConfig(ctx context.Context, client client.Client) (*apiv1alpha1.Config, error) {

	config := apiv1alpha1.Config{}
	configs := apiv1alpha1.ConfigList{}

	clusterConfigs := apiv1alpha1.ClusterConfigList{}

	noNamespaceConfig := false
	// list all apiv1alpha1.Config from the current namespace
	if err := client.List(ctx, &configs); err != nil {
		log.Log.Error(err, "ConfigController - Unable to list Configs in the current namespace")
		noNamespaceConfig = true
		// return &config, err
	} else {
		if len(configs.Items) == 0 {
			noNamespaceConfig = true
		} else {
			// Check if configs has more than one object
			if len(configs.Items) > 1 {
				log.Log.Info("ConfigController - More than one config.api.syncsecretakv.io object found for the current namespace, using the first object and ignoring the rest.")
				config = configs.Items[0]
			} else {
				config = configs.Items[0]
			}
		}
	}

	if noNamespaceConfig {
		if err := client.List(ctx, &clusterConfigs); err != nil {
			log.Log.Error(err, "ConfigController - Unable to list ClusterConfig in the Cluster")
			return nil, err
		} else {
			if len(clusterConfigs.Items) == 0 {
				log.Log.Info("ConfigController - No Namespace Config found or ClusterConfig in the cluster. Do nothing.")
				err := errors.New("no namespace nonfig found or nlusterconfig in the cluster, do nothing")
				return nil, err
			}

			if len(clusterConfigs.Items) > 1 {
				log.Log.Info("ConfigController - More than one clusterconfig.api.syncsecretakv.io object found in the cluster, using the first object and ignoring the rest.")
				//config = configs.Items[0]
				config = *ConvertToConfig(&clusterConfigs.Items[0])
			} else {
				config = *ConvertToConfig(&clusterConfigs.Items[0])
			}
		}
	}

	return &config, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.Config{}).
		Complete(r)
}
