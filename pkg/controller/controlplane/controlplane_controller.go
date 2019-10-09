package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	awsHelper "github.com/awslabs/aws-eks-cluster-controller/pkg/aws"
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
)

var (
	StatusCreateComplete = "Complete"
	StatusCreating       = "Creating"
	StatusUpdating       = "Updating"
	StatusFailed         = "Failed"
	StatusError          = "Error"
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

	stackName := eksCluster.GetControlPlaneStackName()
	finalizer := "cfn-stack.controlplane.eks.amazonaws.com"
	logger = logger.With(
		zap.String("AWSAccountID", eksCluster.Spec.AccountID),
		zap.String("AWSRegion", eksCluster.Spec.Region),
		zap.String("StackName", stackName),
	)

	stack, err := awsHelper.DescribeStack(cfnSvc, stackName)

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !finalizers.HasFinalizer(instance, finalizer) {
			logger.Info("adding finalizer", zap.String("Finalizer", finalizer))
			instance.SetFinalizers(finalizers.AddFinalizer(instance, finalizer))
		}
	} else {
		if finalizers.HasFinalizer(instance, finalizer) {
			logger.Info("deleting control plane cloudformation stack")

			if err != nil && awsHelper.IsStackDoesNotExist(err) {
				logger.Info("stack does not exist, removing finalizer", zap.String("Finalizer", finalizer))
				instance.SetFinalizers(finalizers.RemoveFinalizer(instance, finalizer))
				return reconcile.Result{}, r.Update(context.TODO(), instance)
			}
			if err != nil {
				r.fail(instance, "error deleting controlplane cloudformation stack", err, logger)
				return reconcile.Result{}, err
			}

			if *stack.StackStatus == cloudformation.StackStatusDeleteComplete {
				logger.Info("stack deleted, removing finalizer", zap.String("Finalizer", finalizer))
				instance.SetFinalizers(finalizers.RemoveFinalizer(instance, finalizer))
				return reconcile.Result{}, r.Update(context.TODO(), instance)
			}

			if *stack.StackStatus == cloudformation.StackStatusDeleteInProgress {
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
			}

			_, err = cfnSvc.DeleteStack(&cloudformation.DeleteStackInput{
				StackName: aws.String(stackName),
			})
			if err != nil {
				r.fail(instance, "error deleting controlplane cloudformation stack", err, logger)
				return reconcile.Result{}, err
			}
			return reconcile.Result{Requeue: true}, nil
		}
		// instance is deleted, but nothing to do.
		return reconcile.Result{}, nil
	}

	crdParameters := parseCFNParameterFromCRD(instance)

	if err != nil && awsHelper.IsStackDoesNotExist(err) {
		logger.Info("creating stack")

		err = r.createControlPlaneStack(cfnSvc, stackName, instance)
		if err != nil {
			r.fail(instance, "error creating controlplane cloudformation stack", err, logger)
			return reconcile.Result{}, err
		}

		logger.Info("cloudformation stack created successfully")
		instance.Status.Status = StatusCreating
		instance.SetFinalizers(finalizers.AddFinalizer(instance, finalizer))
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		r.fail(instance, "error describing stack", err, logger)
		return reconcile.Result{}, err
	}

	cfnParameters, err := getCFNParametersFromCFNStack(cfnSvc, stackName)
	if err != nil {
		r.fail(instance, "error trying to read the CFN Parameters", err, logger)
		return reconcile.Result{}, err
	}

	if shouldUpdate(crdParameters, cfnParameters) {
		logger.Info("Updating the control Plane")

		err := r.updateControlPlaneStack(cfnSvc, stackName, instance)
		if err != nil {
			r.fail(instance, "error updating the controlplane cloudformation stack", err, logger)
			return reconcile.Result{}, err
		}

		instance.Status.Status = StatusUpdating
		r.Update(context.TODO(), instance)

		return reconcile.Result{Requeue: true}, nil
	}

	if awsHelper.IsPending(*stack.StackStatus) {
		logger.Info("waiting for stack to complete", zap.String("StackStatus", *stack.StackStatus))
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if awsHelper.IsComplete(*stack.StackStatus) {
		logger.Info("stack complete", zap.String("StackStatus", *stack.StackStatus))
		instance.Status.Status = StatusCreateComplete
		return reconcile.Result{}, r.Update(context.TODO(), instance)
	}

	if awsHelper.IsFailed(*stack.StackStatus) {
		logger.Info("stack in failed state", zap.String("StackStatus", *stack.StackStatus))
		instance.Status.Status = StatusFailed
		return reconcile.Result{}, r.Update(context.TODO(), instance)
	}

	r.fail(instance, "stack in unexpected state", err, logger)
	return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *ReconcileControlPlane) fail(instance *clusterv1alpha1.ControlPlane, msg string, err error, logger *zap.Logger) {
	logger.Error(msg, zap.Error(err))
	instance.Status.Status = StatusFailed
	r.Update(context.TODO(), instance)
}

type controlPlaneTemplateInput struct {
	ClusterName string
	Version     string
	Network     clusterv1alpha1.Network
}

func (r *ReconcileControlPlane) createControlPlaneStack(cfnSvc cloudformationiface.CloudFormationAPI, stackName string, instance *clusterv1alpha1.ControlPlane) error {
	network, err := instance.GetNetwork()
	if err != nil {
		return err
	}

	body, err := awsHelper.GetCFNTemplateBody(controlplaneCFNTemplate, controlPlaneTemplateInput{
		ClusterName: instance.Spec.ClusterName,
		Version:     instance.GetVersion(),
		Network:     network,
	})
	if err != nil {
		return err
	}

	_, err = cfnSvc.CreateStack(&cloudformation.CreateStackInput{
		TemplateBody: aws.String(body),
		StackName:    aws.String(stackName),
		Capabilities: []*string{aws.String("CAPABILITY_IAM")},
		Tags: []*cloudformation.Tag{
			{
				Key:   aws.String("ClusterName"),
				Value: aws.String(instance.Spec.ClusterName),
			},
		},
	})
	return err

}

func (r *ReconcileControlPlane) updateControlPlaneStack(cfnSvc cloudformationiface.CloudFormationAPI, stackName string, instance *clusterv1alpha1.ControlPlane) error {
	network, err := instance.GetNetwork()
	if err != nil {
		return err
	}

	body, err := awsHelper.GetCFNTemplateBody(controlplaneCFNTemplate, controlPlaneTemplateInput{
		ClusterName: instance.Spec.ClusterName,
		Version:     instance.GetVersion(),
		Network:     network,
	})
	if err != nil {
		return err
	}

	_, err = cfnSvc.UpdateStack(&cloudformation.UpdateStackInput{
		TemplateBody: aws.String(body),
		StackName:    aws.String(stackName),
		Capabilities: []*string{aws.String("CAPABILITY_IAM")},
		Tags: []*cloudformation.Tag{
			{
				Key:   aws.String("ClusterName"),
				Value: aws.String(instance.Spec.ClusterName),
			},
		},
	})
	return err

}

func parseCFNParameterFromCRD(cp *clusterv1alpha1.ControlPlane) []*cloudformation.Parameter {
	var params []*cloudformation.Parameter

	if cp.Spec.Version != nil {
		params = append(params, &cloudformation.Parameter{
			ParameterKey:   aws.String("EKSVersion"),
			ParameterValue: aws.String(cp.GetVersion()),
		})
	}

	return params
}

func getCFNParametersFromCFNStack(cfnSvc cloudformationiface.CloudFormationAPI, stackName string) ([]*cloudformation.Parameter, error) {

	output, err := cfnSvc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})

	if err != nil {
		return nil, err
	}

	if len(output.Stacks) != 1 {
		return nil, fmt.Errorf("Error while describing the stacks, got %d stacks for the name : %s", len(output.Stacks), stackName)
	}

	return output.Stacks[0].Parameters, nil
}

func shouldUpdate(crdParams []*cloudformation.Parameter, cfnParams []*cloudformation.Parameter) bool {
	cfnParamMap := make(map[string]string)
	for _, param := range cfnParams {
		cfnParamMap[*param.ParameterKey] = *param.ParameterValue
	}

	for _, param := range crdParams {
		if cfnParamMap[*param.ParameterKey] != *param.ParameterValue {
			return true
		}
	}
	return false
}
