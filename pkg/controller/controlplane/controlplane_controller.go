package controlplane

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/cfnhelper"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/finalizers"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"text/template"
)

var (
	StatusCreateComplete = "Create Control Plane Complete"
	StatusCreateFailed   = "Create Control Plane Failed"
	StatusDeleteFailed   = "Delete Control Plane Failed"
	StatusError          = "Control Plane Error"
	FinalizerCFNStack    = "cfn-stack.controlplane.eks.amazonaws.com"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ControlPlane Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this cluster.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileControlPlane{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		sess:   session.Must(session.NewSession()),
		cfnSvc: nil,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("controlplane-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: 5})
	if err != nil {
		return err
	}

	// Watch for changes to ControlPlane
	err = c.Watch(&source.Kind{Type: &clusterv1alpha1.ControlPlane{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by ControlPlane - change this for objects you create

	return nil
}

var _ reconcile.Reconciler = &ReconcileControlPlane{}

// ReconcileControlPlane reconciles a ControlPlane object
type ReconcileControlPlane struct {
	client.Client
	scheme *runtime.Scheme
	log    *zap.Logger
	sess   *session.Session
	cfnSvc cloudformationiface.CloudFormationAPI
}

// Reconcile reads that state of the cluster for a ControlPlane object and makes changes based on the state read
// and what is in the ControlPlane.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.eks.amazonaws.com,resources=controlplanes,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileControlPlane) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the ControlPlane instance
	logger := r.log.With(
		zap.String("Kind", "ControlPlane"),
		zap.String("Name", request.Name),
		zap.String("NameSpace", request.Namespace),
	)

	instance := &clusterv1alpha1.ControlPlane{}
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

	stackName := fmt.Sprintf("eks-%s", instance.Spec.ClusterName)

	labelsToVerify := []string{"eks.owner.name", "eks.owner.namespace"}
	for _, label := range labelsToVerify {
		if _, ok := instance.Labels[label]; !ok {
			logger.Error("label is missing", zap.String("LabelKey", label))
			instance.Status.Status = StatusError
			r.Update(context.TODO(), instance)
			return reconcile.Result{}, fmt.Errorf("%s label is missing from control plane", label)
		}
	}
	eksCluster := &clusterv1alpha1.EKS{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: instance.Labels["eks.owner.name"], Namespace: instance.Labels["eks.owner.namespace"]}, eksCluster)
	if err != nil {
		logger.Error("EKS cluster not found", zap.Error(err))
		instance.Status.Status = StatusError
		r.Update(context.TODO(), instance)
		return reconcile.Result{}, err
	}
	logger.Info("found cluster", zap.String("ClusterName", eksCluster.Name))

	var cfnSvc cloudformationiface.CloudFormationAPI
	if r.cfnSvc == nil {
		targetAccountSession, err := eksCluster.Spec.GetCrossAccountSession(r.sess)
		if err != nil {
			logger.Error("failed to get cross account session", zap.Error(err))
			instance.Status.Status = StatusError
			r.Update(context.TODO(), instance)
			return reconcile.Result{}, err
		}

		cfnSvc = cloudformation.New(targetAccountSession)
	} else {
		cfnSvc = r.cfnSvc
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if finalizers.HasFinalizer(instance, FinalizerCFNStack) {
			logger.Info("deleting control plane cloudformation stack", zap.String("AWSAccountID", eksCluster.Spec.AccountID), zap.String("AWSRegion", eksCluster.Spec.Region), zap.String("StackName", stackName))

			err = cfnhelper.DeleteStack(cfnSvc, stackName)
			if err != nil {
				logger.Error("error deleting controlplane cloudformation stack", zap.Error(err))
				instance.Status.Status = StatusDeleteFailed
				r.Update(context.TODO(), instance)
				return reconcile.Result{}, err
			}
			logger.Info("cloudformation stack deleted successfully", zap.String("StackName", stackName))

			instance.SetFinalizers(finalizers.RemoveFinalizer(instance, FinalizerCFNStack))
			err = r.Update(context.TODO(), instance)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{Requeue: true}, nil
	}

	if instance.Status.Status == StatusCreateComplete || instance.Status.Status == StatusCreateFailed {
		// TODO: verify update is needed or not and take action
		// Use cases for update:
		//  Cloudformation template changes - if change happens in cfn_template.go variable how to update all clusters or put template in conigmap
		//  Changes to parameters passed to cfn stack - For just cluster-name change??
		return reconcile.Result{}, nil
	}

	logger.Info("creating EKS control plane cloudformation stack", zap.String("AWSAccountID", eksCluster.Spec.AccountID), zap.String("AWSRegion", eksCluster.Spec.Region), zap.String("StackName", stackName))

	err = r.createControlPlaneStack(cfnSvc, stackName, instance.Spec.ClusterName)
	if err != nil {
		logger.Error("error creating controlplane cloudformation stack", zap.Error(err))
		instance.Status.Status = StatusCreateFailed
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}

	logger.Info("cloudformation stack created successfully", zap.String("StackName", stackName))
	instance.Status.Status = StatusCreateComplete
	instance.SetFinalizers(finalizers.AddFinalizer(instance, FinalizerCFNStack))
	err = r.Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileControlPlane) createControlPlaneStack(cfnSvc cloudformationiface.CloudFormationAPI, stackName, clusterName string) error {
	controlPlaneTemplate, err := template.New("cfntemplate").Parse(controlplaneCFNTemplate)
	if err != nil {
		return err
	}

	b := bytes.NewBuffer([]byte{})
	if err := controlPlaneTemplate.Execute(b, map[string]string{
		"ClusterName": clusterName,
	}); err != nil {
		return err
	}

	_, err = cfnhelper.CreateAndDescribeStack(cfnSvc, &cloudformation.CreateStackInput{
		TemplateBody: aws.String(b.String()),
		StackName:    aws.String(stackName),
		Capabilities: []*string{aws.String("CAPABILITY_IAM")},
		Tags: []*cloudformation.Tag{
			{
				Key:   aws.String("ClusterName"),
				Value: aws.String(clusterName),
			},
		},
	})
	if err != nil {
		return err
	}
	return nil
}
