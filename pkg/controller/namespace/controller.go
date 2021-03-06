/*
Copyright 2018 ReactiveOps.

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

package namespace

import (
	"context"

	rbacmanagerv1beta1 "github.com/reactiveops/rbac-manager/pkg/apis/rbacmanager/v1beta1"
	"github.com/reactiveops/rbac-manager/pkg/controller/rbacdefinition"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Namespace Controller and adds it to the Manager.
// The Manager will set fields on the Controller and Start it.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNamespace{Client: mgr.GetClient(), config: mgr.GetConfig(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("namespace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Namespaces
	err = c.Watch(&source.Kind{Type: &v1.Namespace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// ReconcileNamespace reconciles a Namespace object
type ReconcileNamespace struct {
	client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile makes changes in response to Namespace changes
func (r *ReconcileNamespace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var err error

	// Fetch the Namespace
	namespace := &v1.Namespace{}
	err = r.Get(context.TODO(), request.NamespacedName, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			err = reconcileNamespace(r.config, namespace)
			if err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	err = reconcileNamespace(r.config, namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func reconcileNamespace(config *rest.Config, namespace *v1.Namespace) error {
	var err error
	var rbacDefList rbacmanagerv1beta1.RBACDefinitionList
	rdr := rbacdefinition.Reconciler{}

	// Full Kubernetes ClientSet is required because RBAC types don't
	//   implement methods required for Kubebuilder methods to work
	rdr.Clientset, err = kubernetes.NewForConfig(config)

	if err != nil {
		return err
	}

	rbacDefList, err = getRbacDefinitions(config)

	for _, rbacDef := range rbacDefList.Items {
		err = rdr.ReconcileNamespaceChange(&rbacDef, namespace)
		if err != nil {
			return err
		}
	}

	return nil
}

func getRbacDefinitions(config *rest.Config) (rbacmanagerv1beta1.RBACDefinitionList, error) {
	list := rbacmanagerv1beta1.RBACDefinitionList{}

	rbacmanagerv1beta1.AddToScheme(scheme.Scheme)
	clientConfig := *config
	clientConfig.ContentConfig.GroupVersion = &rbacmanagerv1beta1.SchemeGroupVersion
	clientConfig.APIPath = "/apis"
	clientConfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	clientConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.UnversionedRESTClientFor(&clientConfig)

	if err != nil {
		return list, err
	}

	err = client.Get().Resource("rbacdefinitions").Do().Into(&list)

	return list, err
}
