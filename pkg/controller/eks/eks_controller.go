package eks

import (
	"context"
	"fmt"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"

	"go.uber.org/zap"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/controller/controlplane"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/finalizers"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new EKS Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this cluster.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileEKS{Client: mgr.GetClient(), scheme: mgr.GetScheme(), log: logging.New()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("eks-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 10})
	if err != nil {
		return err
	}

	// Watch for changes to EKS
	err = c.Watch(&source.Kind{Type: &clusterv1alpha1.EKS{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{
		Type: &clusterv1alpha1.ControlPlane{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &clusterv1alpha1.EKS{},
		})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{
		Type: &clusterv1alpha1.NodeGroup{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &clusterv1alpha1.EKS{},
		})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileEKS{}

var (
	NodeGroupFinalizer    = "nodegroup.eks.amazonaws.com"
	ControlPlaneFinalizer = "controlplane.eks.amazonaws.com"
)

// ReconcileEKS reconciles a EKS object
type ReconcileEKS struct {
	client.Client
	scheme *runtime.Scheme
	log    *zap.Logger
}

// Reconcile reads that state of the cluster for a EKS object and makes changes based on the state read
// and what is in the EKS.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=controlplane,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=nodegroup,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=eks,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileEKS) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the EKS instance
	logger := r.log.With(
		zap.String("Kind", "EKS"),
		zap.String("Name", request.Name),
		zap.String("NameSpace", request.Namespace),
	)
	defer logger.Sync()

	instance := &clusterv1alpha1.EKS{}
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

	logger.Info("got reconcile request")
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("deleting eks")
		if finalizers.HasFinalizer(instance, NodeGroupFinalizer) {
			result, err := r.deleteNodeGroups(instance, logger)
			if err != nil {
				logger.Error("Error Deleting Node Groups", zap.Error(err))
			}
			return result, err
		}

		if finalizers.HasFinalizer(instance, ControlPlaneFinalizer) {
			result, err := r.deleteControlPlane(instance, logger)
			if err != nil {
				logger.Error("Error Deleting Control Plane", zap.Error(err))
			}
			return result, err
		}

		return reconcile.Result{}, nil
	}

	cp, err := r.createControlPlane(instance)
	if err != nil {
		logger.Error("create Control Plane Failed", zap.Error(err))
		return reconcile.Result{}, err
	}
	if cp == nil || cp.Status.Status != controlplane.StatusComplete {
		logger.Info("create Control Plane not complete")
		return reconcile.Result{}, nil
	}
	logger.Info("create Control Plane complete", zap.String("ControlPlaneStatus", cp.Status.Status))

	logger.Info("creating NodeGroups")
	ngl, err := r.createAllNodeGroups(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ngl == nil || len(ngl) != len(instance.Spec.NodeGroups) {
		return reconcile.Result{}, nil
	}

	ngNames := make([]string, 0, len(ngl))
	for _, ng := range ngl {
		ngNames = append(ngNames, ng.GetName())
	}
	logger.Info("create NodeGroups complete", zap.Strings("NodeGroups", ngNames))

	instance.Status.Status = "Complete"

	err = r.Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileEKS) createControlPlane(instance *clusterv1alpha1.EKS) (*clusterv1alpha1.ControlPlane, error) {
	cp := &clusterv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", instance.Name, "controlplane"),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"eks.owner": fmt.Sprintf("%s", instance.Name),
			},
		},
		Spec: clusterv1alpha1.ControlPlaneSpec{
			ClusterName: instance.Spec.ControlPlane.ClusterName,
			StackName:   instance.Spec.ControlPlane.StackName,
		},
	}

	logger := r.log.With(
		zap.String("Kind", "EKS"),
		zap.String("Name", instance.Name),
		zap.String("NameSpace", instance.Namespace),
	)

	found := &clusterv1alpha1.ControlPlane{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: cp.Name, Namespace: cp.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		if err := controllerutil.SetControllerReference(instance, cp, r.scheme); err != nil {
			return nil, err
		}

		instance.SetFinalizers(finalizers.AddFinalizer(instance, ControlPlaneFinalizer))

		logger.Info("creating Control Plane")
		err = r.Create(context.TODO(), cp)
		if err != nil {
			return nil, err
		}

		instance.Status.Status = controlplane.StatusCreating
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return nil, err
		}
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return found, nil
}

func (r *ReconcileEKS) createAllNodeGroups(instance *clusterv1alpha1.EKS) ([]*clusterv1alpha1.NodeGroup, error) {
	logger := r.log.With(
		zap.String("Kind", "EKS"),
		zap.String("Name", instance.Name),
		zap.String("NameSpace", instance.Namespace),
	)

	nodeGroups := make([]*clusterv1alpha1.NodeGroup, 0, len(instance.Spec.NodeGroups))
	creating := false
	for _, nodeGroupSpec := range instance.Spec.NodeGroups {
		nodeGroup := &clusterv1alpha1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-nodegroup-%s", instance.Name, nodeGroupSpec.Name),
				Namespace: instance.Namespace,
				Labels: map[string]string{
					"eks.owner": fmt.Sprintf("%s_%s", instance.Namespace, instance.Name),
				},
			},
			Spec: nodeGroupSpec,
		}

		if err := controllerutil.SetControllerReference(instance, nodeGroup, r.scheme); err != nil {
			return nil, err
		}

		found := &clusterv1alpha1.NodeGroup{}
		err := r.Get(context.TODO(), types.NamespacedName{Name: nodeGroup.Name, Namespace: nodeGroup.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {

			instance.SetFinalizers(finalizers.AddFinalizer(instance, NodeGroupFinalizer))

			logger.Info("creating NodeGroup", zap.String("NodeGroup", nodeGroup.Name))
			err = r.Create(context.TODO(), nodeGroup)
			if err != nil {
				return nil, err
			}

			creating = true
		} else if err != nil {
			return nil, err
		}
		nodeGroups = append(nodeGroups, found)
	}
	if creating {
		instance.Status.Status = "Creating Control Plane"
		err := r.Update(context.TODO(), instance)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	return nodeGroups, nil
}

func (r *ReconcileEKS) deleteNodeGroups(instance *clusterv1alpha1.EKS, logger *zap.Logger) (reconcile.Result, error) {

	nodeGroups := &clusterv1alpha1.NodeGroupList{}

	err := r.List(context.TODO(),
		client.MatchingLabels(map[string]string{
			"eks.owner": fmt.Sprintf("%s_%s", instance.Namespace, instance.Name),
		}),
		nodeGroups)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(nodeGroups.Items) == 0 {
		logger.Info("Node Groups removed")
		instance.Finalizers = finalizers.RemoveFinalizer(instance, NodeGroupFinalizer)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	for _, nodeGroup := range nodeGroups.Items {
		logger.Info("deleting Node Group", zap.String("NodeGroup", nodeGroup.Name))
		err = r.Delete(context.TODO(), &nodeGroup)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{Requeue: true}, nil
}

func (r *ReconcileEKS) deleteControlPlane(instance *clusterv1alpha1.EKS, logger *zap.Logger) (reconcile.Result, error) {
	controlPlanes := &clusterv1alpha1.ControlPlaneList{}

	err := r.List(context.TODO(),
		client.MatchingLabels(map[string]string{
			"eks.owner": fmt.Sprintf("%s", instance.Name),
		}),
		controlPlanes)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(controlPlanes.Items) == 0 {
		logger.Info("Control Plane removed")
		instance.Finalizers = finalizers.RemoveFinalizer(instance, ControlPlaneFinalizer)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	for _, controlPlane := range controlPlanes.Items {
		logger.Info("deleting Control Plane", zap.String("ControlPlane", controlPlane.Name))

		err = r.Delete(context.TODO(), &controlPlane)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	instance.Status.Status = controlplane.StatusDeleting
	err = r.Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true}, nil
}
