package authorizer

import (
	"github.com/awslabs/aws-eks-cluster-controller/pkg/apis"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
)

type Authorizer interface {
	GetKubeConfig(*clusterv1alpha1.EKS) ([]byte, error)
	GetClient(*clusterv1alpha1.EKS) (client.Client, error)
}

func GetClientFromConfig(config []byte) (client.Client, error) {
	// Add our custom resources to the client
	s := scheme.Scheme
	if err := apis.AddToScheme(s); err != nil {
		return nil, err
	}

	cc, err := clientcmd.NewClientConfigFromBytes(config)
	if err != nil {
		return nil, err
	}
	restConfig, err := cc.ClientConfig()
	if err != nil {
		return nil, err
	}
	return client.New(restConfig, client.Options{Scheme: s})
}
