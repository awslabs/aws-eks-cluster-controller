package eks

import (
	"context"
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ControlPlaneCreating = "Creating"
	ControlPlaneUpdating = "Updating"
)

func createOrUpdateControlPlane(cp *clusterv1alpha1.ControlPlane, c client.Client) (string, error) {

	found := &clusterv1alpha1.ControlPlane{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: cp.Name, Namespace: cp.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return ControlPlaneCreating, c.Create(context.TODO(), cp)
	} else if err != nil {
		return "", err
	}

	if !reflect.DeepEqual(cp.Spec, found.Spec) {
		found.Spec = cp.Spec
		return ControlPlaneUpdating, c.Update(context.TODO(), found)
	}
	return found.Status.Status, nil
}
