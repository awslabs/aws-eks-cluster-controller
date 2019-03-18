package authorizer

import (
	"log"

	"github.com/awslabs/aws-eks-cluster-controller/pkg/apis"
	apiextv1beta "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
)

func init() {
	// Add our custom resources to the client
	if err := apis.AddToScheme(scheme.Scheme); err != nil {
		log.Panic(err)
	}
	if err := apiextv1beta.AddToScheme(scheme.Scheme); err != nil {
		log.Panic(err)
	}
}

type Authorizer interface {
	GetKubeConfig(*clusterv1alpha1.EKS) ([]byte, error)
	GetClient(*clusterv1alpha1.EKS) (client.Client, error)
}

func GetClientFromConfig(config []byte) (client.Client, error) {
	s := scheme.Scheme
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
