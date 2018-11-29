package authorizer

import (
	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeAuthorizer struct {
	client client.Client
}

var _ Authorizer = &FakeAuthorizer{}

func NewFake(c client.Client) *FakeAuthorizer {
	return &FakeAuthorizer{
		client: c,
	}
}

func (f *FakeAuthorizer) GetKubeConfig(e *clusterv1alpha1.EKS) ([]byte, error) {
	return []byte{}, nil
}
func (f *FakeAuthorizer) GetClient(e *clusterv1alpha1.EKS) (client.Client, error) {
	return f.client, nil
}
