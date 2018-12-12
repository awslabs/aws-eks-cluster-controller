package eks

import (
	"testing"
	"time"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	componentsv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/components/v1alpha1"

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

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var controlPlaneKey = types.NamespacedName{Name: "foo-controlplane", Namespace: "default"}
var nodeGroup1Key = types.NamespacedName{Name: "foo-nodegroup-group1", Namespace: "default"}
var nodeGroup2Key = types.NamespacedName{Name: "foo-nodegroup-group2", Namespace: "default"}
var configmapKey = types.NamespacedName{Name: "foo-configmap-aws-auth", Namespace: "default"}
var eksKey = types.NamespacedName{Name: "foo", Namespace: "default"}

const timeout = time.Second * 5

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	instance := &clusterv1alpha1.EKS{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: clusterv1alpha1.EKSSpec{
			AccountID: "1234foo",
			ControlPlane: clusterv1alpha1.ControlPlaneSpec{
				ClusterName: "cluster-stuff",
			},
			NodeGroups: []clusterv1alpha1.NodeGroupSpec{
				clusterv1alpha1.NodeGroupSpec{Name: "Group1"},
				clusterv1alpha1.NodeGroupSpec{Name: "Group2"},
			},
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Create the EKS object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	//go func() {
	//	for range requests {
	//	}
	//}()

	controlPlane := &clusterv1alpha1.ControlPlane{}
	g.Eventually(func() error { return c.Get(context.TODO(), controlPlaneKey, controlPlane) }, timeout).
		Should(gomega.Succeed())

	// The ControlPlane controller is not running so set it's status manually
	controlPlane.Status.Status = "Complete"
	g.Expect(c.Update(context.TODO(), controlPlane)).Should(gomega.Succeed())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	nodeGroup1 := &clusterv1alpha1.NodeGroup{}
	g.Eventually(func() error { return c.Get(context.TODO(), nodeGroup1Key, nodeGroup1) }, timeout).
		Should(gomega.Succeed())
	nodeGroup1.Status.Status = "Complete"
	g.Expect(c.Update(context.TODO(), nodeGroup1)).Should(gomega.Succeed())

	nodeGroup2 := &clusterv1alpha1.NodeGroup{}
	g.Eventually(func() error { return c.Get(context.TODO(), nodeGroup2Key, nodeGroup2) }, timeout).
		Should(gomega.Succeed())
	nodeGroup2.Status.Status = "Complete"
	g.Expect(c.Update(context.TODO(), nodeGroup2)).Should(gomega.Succeed())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	configmap := &componentsv1alpha1.ConfigMap{}
	g.Eventually(func() error { return c.Get(context.TODO(), configmapKey, configmap) }, timeout).
		Should(gomega.Succeed())

	g.Eventually(func() (string, error) {
		eks := &clusterv1alpha1.EKS{}
		err := c.Get(context.TODO(), eksKey, eks)
		return eks.Status.Status, err
	}).Should(gomega.Equal("Complete"))

	err = c.Delete(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Drain requests.  The delete function loops a number of times
	go func() {
		for range requests {
		}
	}()
	g.Eventually(func() error { return c.Get(context.TODO(), configmapKey, configmap) }).ShouldNot(gomega.Succeed())
	g.Eventually(func() error { return c.Get(context.TODO(), nodeGroup1Key, nodeGroup1) }).ShouldNot(gomega.Succeed())
	g.Eventually(func() error { return c.Get(context.TODO(), nodeGroup2Key, nodeGroup2) }).ShouldNot(gomega.Succeed())
	g.Eventually(func() error { return c.Get(context.TODO(), controlPlaneKey, controlPlane) }).ShouldNot(gomega.Succeed())

	g.Eventually(func() error { return c.Get(context.TODO(), eksKey, instance) }).ShouldNot(gomega.Succeed())

}
