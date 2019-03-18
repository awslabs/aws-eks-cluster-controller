// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package customresourcedefinition

import (
	"context"
	"reflect"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/finalizers"

	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/authorizer"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"
	"go.uber.org/zap"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	componentsv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/components/v1alpha1"
	apiextv1beta "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// statuses and finalizers
var (
	CRDFinalizer        = "crd.components.eks.amazon.com"
	RemoteObjectCreated = "Created"
)

// Add creates a new CustomResourceDefinition Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	sess := session.Must(session.NewSession())
	log := logging.New()
	return &ReconcileCustomResourceDefinition{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    log,
		sess:   sess,
		auth:   authorizer.NewEks(sess, log),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("customresourcedefinition-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to CustomResourceDefinition
	err = c.Watch(&source.Kind{Type: &componentsv1alpha1.CustomResourceDefinition{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileCustomResourceDefinition{}

// ReconcileCustomResourceDefinition reconciles a CustomResourceDefinition object
type ReconcileCustomResourceDefinition struct {
	client.Client
	scheme *runtime.Scheme
	log    *zap.Logger
	sess   *session.Session
	auth   authorizer.Authorizer
}

// Reconcile reads that state of the cluster for a CustomResourceDefinition object and makes changes based on the state read
// and what is in the CustomResourceDefinition.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=components.eks.amazonaws.com,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=components.eks.amazonaws.com,resources=customresourcedefinitions/status,verbs=get;update;patch
func (r *ReconcileCustomResourceDefinition) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := r.log.With(
		zap.String("Name", request.Name),
		zap.String("Namespace", request.Namespace),
		zap.String("Kind", "customresourcedefinitions.components.eks.amazonaws.com"),
	)
	// Fetch the CustomResourceDefinition instance
	instance := &componentsv1alpha1.CustomResourceDefinition{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	remoteKey := types.NamespacedName{Name: instance.Spec.Name}

	cluster := &clusterv1alpha1.EKS{}
	clusterKey := types.NamespacedName{Name: instance.Spec.Cluster, Namespace: instance.Namespace}
	if err := r.Get(context.TODO(), clusterKey, cluster); err != nil {
		instance.Finalizers = []string{}
		instance.Status.Status = "EKS Cluster not found"
		log.Error("EKS cluster not found", zap.Error(err))
		r.Update(context.TODO(), instance)
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
	log.Info("got cluster", zap.String("ClusterName", cluster.Name))

	if err := controllerutil.SetControllerReference(cluster, instance, r.scheme); err != nil {
		return reconcile.Result{}, nil
	}

	client, err := r.auth.GetClient(cluster)
	if err != nil {
		log.Error("could not access remote cluster", zap.Error(err))
		return reconcile.Result{}, err
	}

	log.Info("got the remote client")

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if finalizers.HasFinalizer(instance, CRDFinalizer) {
			log.Info("deleting crds")
			crdFound := &apiextv1beta.CustomResourceDefinition{}
			err := client.Get(context.TODO(), remoteKey, crdFound)
			if err != nil && errors.IsNotFound(err) {
				log.Info("remote crd not found removing the finalizers")
				instance.Finalizers = finalizers.RemoveFinalizer(instance, CRDFinalizer)
				if err := r.Client.Update(context.TODO(), instance); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, nil
			} else if err != nil {
				log.Error("could not get remote crd", zap.Error(err))
				return reconcile.Result{}, nil
			}

			if err := client.Delete(context.TODO(), crdFound); err != nil {
				log.Error("could not delete remote crd", zap.Error(err))
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, nil
	}

	crdExpected := &apiextv1beta.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Spec.Name,
			Labels:      instance.Labels,
			Annotations: instance.Annotations,
		},
		Spec: instance.Spec.CustomResourceDefinitionSpec,
	}
	crdFound := &apiextv1beta.CustomResourceDefinition{}
	err = client.Get(context.TODO(), remoteKey, crdFound)
	if err != nil && errors.IsNotFound(err) {
		log.Info("creating crd")

		if err := client.Create(context.TODO(), crdExpected); err != nil {
			log.Error("failed to create the remote crd", zap.Error(err))
			return reconcile.Result{}, err
		}
		instance.Finalizers = []string{CRDFinalizer}
		instance.Status.Status = RemoteObjectCreated

		if err = r.Client.Update(context.TODO(), instance); err != nil {
			log.Error("failed to update the crd's status", zap.Error(err))
			return reconcile.Result{}, err
		}

		log.Info("successfully created the remote crd")
		return reconcile.Result{}, nil
	} else if err != nil {
		log.Error("error retrieving the remote crd")
		return reconcile.Result{}, err
	}

	log.Info("found the remote crd")
	if !reflect.DeepEqual(crdFound.Spec, crdExpected.Spec) {
		crdFound.Spec = crdExpected.Spec
		log.Info("updating remote crd")
		err := client.Update(context.TODO(), crdFound)
		if err != nil {
			log.Error("unable to update remote crd", zap.Error(err))
			return reconcile.Result{}, err
		}
		log.Info("remote crd updated")
	}
	return reconcile.Result{}, nil
}
