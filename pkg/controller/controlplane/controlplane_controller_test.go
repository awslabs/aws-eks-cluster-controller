package controlplane

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/aws/aws-sdk-go/aws/awserr"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/cfnhelper"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo-cp", Namespace: "default"}}
var cpKey = types.NamespacedName{Name: "foo-cp", Namespace: "default"}

const timeout = time.Second * 10

// newReconciler returns a new reconcile.Reconciler
func newTestReconciler(mgr manager.Manager) *ReconcileControlPlane {
	var errDoesNotExist = awserr.New("ValidationError", "Stack with id eks-foo-cluster does not exist", nil)
	return &ReconcileControlPlane{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		cfnSvc: &cfnhelper.MockCloudformationAPI{FailDescribe: true, Err: errDoesNotExist},
	}
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cluster := &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-eks", Namespace: "default"},
		Spec: clusterv1alpha1.EKSSpec{
			AccountID: "1234",
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "foo-cluster",
			},
			CrossAccountRoleName: "foo-role",
			NodeGroups:           []clusterv1alpha1.NodeGroupSpec{{Name: "foo-ng"}},
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
	c = mgr.GetClient()

	reconciler := newTestReconciler(mgr)
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

	reconciler.cfnSvc = &cfnhelper.MockCloudformationAPI{Status: cloudformation.StackStatusCreateComplete}
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	g.Eventually(func() (string, error) {
		err := c.Get(context.TODO(), cpKey, getCP)
		return getCP.Status.Status, err
	}).Should(gomega.Equal(StatusCreateComplete))

	err = c.Delete(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}
