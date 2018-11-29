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
	NodeGroupCreating  = "Creating"
	NodeGroupUpdating  = "Updating"
	NodeGroupCompleted = "Completed"
)

func createOrUpdateNodegroups(nodeGroups []*clusterv1alpha1.NodeGroup, c client.Client) (string, error) {
	createdNG := []*clusterv1alpha1.NodeGroup{}
	foundNG := []*clusterv1alpha1.NodeGroup{}
	for _, nodeGroup := range nodeGroups {

		status, err := createOrUpdateNodegroup(nodeGroup, c)
		if err != nil {
			return "", err
		}
		if status == NodeGroupCreating {
			createdNG = append(createdNG, nodeGroup)
		}
		if status == NodeGroupUpdating {
			foundNG = append(foundNG, nodeGroup)
		}
	}

	if len(createdNG) > 0 {
		return NodeGroupCreating, nil
	}

	for _, nodeGroup := range foundNG {
		if nodeGroup.Status.Status != NodeGroupCompleted {
			return NodeGroupUpdating, nil
		}
	}

	return NodeGroupCompleted, nil

}

func createOrUpdateNodegroup(ng *clusterv1alpha1.NodeGroup, c client.Client) (string, error) {

	found := &clusterv1alpha1.NodeGroup{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: ng.Name, Namespace: ng.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return NodeGroupCreating, c.Create(context.TODO(), ng)
	} else if err != nil {
		return "", err
	}

	if !reflect.DeepEqual(ng.Spec, found.Spec) {
		found.Spec = ng.Spec
		return NodeGroupUpdating, c.Update(context.TODO(), found)
	}
	return found.Status.Status, nil
}
