package nodegroup

import (
	"testing"
	"time"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/cfnhelper"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/logging"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

func getRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}}
}
func getNGKey(name string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: "default"}
}

func getEKSCluster(name string) *clusterv1alpha1.EKS {
	return &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: clusterv1alpha1.EKSSpec{
			AccountID:            "1234foo",
			CrossAccountRoleName: "foo-role",
			Region:               "us-test-1",
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "cluster-stuff",
			},
			NodeGroups: []clusterv1alpha1.NodeGroupSpec{
				clusterv1alpha1.NodeGroupSpec{Name: "NG1"},
			},
		},
	}
}

const timeout = time.Second * 25

func newTestReconciler(mgr manager.Manager, cfn *cfnhelper.MockCloudformationAPI) reconcile.Reconciler {
	return &ReconcileNodeGroup{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		log:    logging.New(),
		sess:   nil,
		cfnSvc: cfn,
	}
}
func TestNodeGroupReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	type testCase struct {
		instance        *clusterv1alpha1.NodeGroup
		expectedStatus  string
		logMessage      string
		searchKey       types.NamespacedName
		expectedRequest reconcile.Request
	}

	var testCases = []*testCase{}

	createIsSuccessful := new(testCase)
	createIsSuccessful.instance = &clusterv1alpha1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-ng1", Namespace: "default", Labels: map[string]string{"eks.owner.name": "eks-foo", "eks.owner.namespace": "default"}},
		Spec: clusterv1alpha1.NodeGroupSpec{
			Name: "foo-ngroup1",
		},
	}
	createIsSuccessful.expectedStatus = StatusCreateComplete
	createIsSuccessful.logMessage = "Create works correctly"
	createIsSuccessful.searchKey = getNGKey("foo-ng1")
	createIsSuccessful.expectedRequest = getRequest("foo-ng1")
	testCases = append(testCases, createIsSuccessful)

	labelIsMissing := new(testCase)
	labelIsMissing.instance = &clusterv1alpha1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-ng2", Namespace: "default", Labels: map[string]string{"eks.owner.name": "eks-foo"}},
		Spec: clusterv1alpha1.NodeGroupSpec{
			Name: "foo-ngroup2",
		},
	}
	labelIsMissing.expectedStatus = StatusError
	labelIsMissing.logMessage = "Missing EKS label throws an Error"
	labelIsMissing.searchKey = getNGKey("foo-ng2")
	labelIsMissing.expectedRequest = getRequest("foo-ng2")
	testCases = append(testCases, labelIsMissing)

	incorrectEKSParent := new(testCase)
	incorrectEKSParent.instance = &clusterv1alpha1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-ng3", Namespace: "default", Labels: map[string]string{"eks.owner.name": "blah-blah", "eks.owner.namespace": "default"}},
		Spec: clusterv1alpha1.NodeGroupSpec{
			Name: "foo-ngroup2",
		},
	}
	incorrectEKSParent.expectedStatus = StatusError
	incorrectEKSParent.logMessage = "Incorrect EKS Owner throws error"
	incorrectEKSParent.searchKey = getNGKey("foo-ng3")
	incorrectEKSParent.expectedRequest = getRequest("foo-ng3")
	testCases = append(testCases, incorrectEKSParent)
	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	cfnSvc := &cfnhelper.MockCloudformationAPI{}
	recFn, requests := SetupTestReconcile(newTestReconciler(mgr, cfnSvc))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	eksFooCluster := getEKSCluster("eks-foo")

	g.Expect(c.Create(context.TODO(), eksFooCluster)).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), eksFooCluster)

	for _, test := range testCases {
		err = c.Create(context.TODO(), test.instance)

		g.Expect(err).NotTo(gomega.HaveOccurred())
		defer c.Delete(context.TODO(), test.instance)
		g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(test.expectedRequest)))

		g.Eventually(func() (string, error) {
			nodegroup := &clusterv1alpha1.NodeGroup{}
			err := c.Get(context.TODO(), test.searchKey, nodegroup)
			return nodegroup.Status.Status, err
		}, timeout).Should(gomega.Equal(test.expectedStatus))

		t.Logf("Test Completed - %s", test.logMessage)
	}
}

func TestNodeGroupCreateFailedSetsCorrectStatus(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	instance := &clusterv1alpha1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-ng4", Namespace: "default", Labels: map[string]string{"eks.owner.name": "eks-foo2", "eks.owner.namespace": "default"}},
		Spec: clusterv1alpha1.NodeGroupSpec{
			Name: "foo-ngroup1",
		},
	}

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	cfnSvc := &cfnhelper.MockCloudformationAPI{}

	cfnSvc.SetCreateStackErr(true)

	recFn, requests := SetupTestReconcile(newTestReconciler(mgr, cfnSvc))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	eksFooCluster := getEKSCluster("eks-foo2")

	g.Expect(c.Create(context.TODO(), eksFooCluster)).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), eksFooCluster)

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(getRequest("foo-ng4"))))

	g.Eventually(func() (string, error) {
		nodegroup := &clusterv1alpha1.NodeGroup{}
		err := c.Get(context.TODO(), getNGKey("foo-ng4"), nodegroup)
		return nodegroup.Status.Status, err
	}, timeout).Should(gomega.Equal(StatusCreateFailed))

}

func TestNodeGroupGetCFNStackNamereturnsExpectedName(t *testing.T) {
	instance := &clusterv1alpha1.NodeGroup{
		Spec: clusterv1alpha1.NodeGroupSpec{
			Name: "foo",
		},
	}
	actual := getCFNStackName(instance)
	expected := "eks-foo"
	if actual != expected {
		t.Errorf("getCFNStackname returned %s for foo", actual)
	}
}
