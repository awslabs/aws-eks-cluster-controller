package controlplane

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/aws/aws-sdk-go/aws/awserr"
	awsHelper "github.com/awslabs/aws-eks-cluster-controller/pkg/aws"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const timeout = time.Second * 10

type cfnSvcConfig struct {
	FailCreate   bool
	FailDescribe bool
	FailDelete   bool
}

func newTestReconciler(mgr manager.Manager, cfnSvcConfig cfnSvcConfig) *ReconcileControlPlane {
	var errDoesNotExist = awserr.New("ValidationError", `ValidationError: Stack with id eks-foo-cluster does not exist, status code: 400, request id: 42`, nil)
	return &ReconcileControlPlane{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		cfnSvc: &awsHelper.MockCloudformationAPI{
			FailCreate:   cfnSvcConfig.FailCreate,
			FailDelete:   cfnSvcConfig.FailDelete,
			FailDescribe: cfnSvcConfig.FailDescribe,
			Err:          errDoesNotExist,
		},
	}
}

func TestReconcileSuccess(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo-cp", Namespace: "default"}}
	cpKey := types.NamespacedName{Name: "foo-cp", Namespace: "default"}

	cluster := &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-eks", Namespace: "default"},
		Spec: clusterv1alpha1.EKSSpec{
			AccountID: "1234",
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "foo-cluster",
			},
			CrossAccountRoleName: "foo-role",
			NodeGroups:           []clusterv1alpha1.NodeGroupSpec{{Name: "foo-ng", IAMPolicies: []clusterv1alpha1.Policy{}}},
			Region:               "us-test-1",
		},
	}

	instance := &clusterv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-cp", Namespace: "default", Labels: map[string]string{"eks.owner.name": "foo-eks", "eks.owner.namespace": "default"}},
		Spec: clusterv1alpha1.ControlPlaneSpec{
			ClusterName: "foo-cluster",
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c := mgr.GetClient()

	reconciler := newTestReconciler(mgr, cfnSvcConfig{FailDescribe: true})
	recFn, requests := SetupTestReconcile(reconciler)
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	g.Expect(c.Create(context.TODO(), cluster)).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), cluster)

	// Create the ControlPlane object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	getCP := &clusterv1alpha1.ControlPlane{}
	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusCreating))

	reconciler.cfnSvc = &awsHelper.MockCloudformationAPI{Status: cloudformation.StackStatusCreateComplete}
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusCreateComplete))

	err = c.Delete(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}

func TestReconcileFail(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"}}
	cpKey := types.NamespacedName{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace"}

	cluster := &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{Name: "test-eks-name", Namespace: "test-controlplane-namespace"},
		Spec: clusterv1alpha1.EKSSpec{
			AccountID: "test-account-id",
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "test-cluster-name",
			},
			CrossAccountRoleName: "test-crossaccount-role",
			NodeGroups:           []clusterv1alpha1.NodeGroupSpec{{Name: "test-nodegroup-name", IAMPolicies: []clusterv1alpha1.Policy{}}},
			Region:               "test-region-id",
		},
	}

	instance := &clusterv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane-name", Namespace: "test-controlplane-namespace", Labels: map[string]string{"eks.owner.name": "test-eks-name", "eks.owner.namespace": "test-controlplane-namespace"}},
		Spec: clusterv1alpha1.ControlPlaneSpec{
			ClusterName: "test-cluster-name",
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c := mgr.GetClient()

	reconciler := newTestReconciler(mgr, cfnSvcConfig{FailDescribe: true})

	recFn, requests := SetupTestReconcile(reconciler)
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	g.Expect(c.Create(context.TODO(), cluster)).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), cluster)

	// Create the ControlPlane object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	getCP := &clusterv1alpha1.ControlPlane{}
	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusCreating))

	reconciler.cfnSvc = &awsHelper.MockCloudformationAPI{Status: cloudformation.StackStatusCreateComplete}
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusCreateComplete))

	// err = c.Delete(context.TODO(), instance)
	// g.Expect(err).NotTo(gomega.HaveOccurred())
	// g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}
