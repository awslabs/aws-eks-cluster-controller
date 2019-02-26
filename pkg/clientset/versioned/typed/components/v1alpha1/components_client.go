/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/components/v1alpha1"
	"github.com/awslabs/aws-eks-cluster-controller/pkg/clientset/versioned/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type ComponentsV1alpha1Interface interface {
	RESTClient() rest.Interface
	ClusterRolesGetter
	ClusterRoleBindingsGetter
	ConfigMapsGetter
	DaemonSetsGetter
	DeploymentsGetter
	IngressesGetter
	SecretsGetter
	ServicesGetter
	ServiceAccountsGetter
}

// ComponentsV1alpha1Client is used to interact with features provided by the components.eks.amazonaws.com group.
type ComponentsV1alpha1Client struct {
	restClient rest.Interface
}

func (c *ComponentsV1alpha1Client) ClusterRoles(namespace string) ClusterRoleInterface {
	return newClusterRoles(c, namespace)
}

func (c *ComponentsV1alpha1Client) ClusterRoleBindings(namespace string) ClusterRoleBindingInterface {
	return newClusterRoleBindings(c, namespace)
}

func (c *ComponentsV1alpha1Client) ConfigMaps(namespace string) ConfigMapInterface {
	return newConfigMaps(c, namespace)
}

func (c *ComponentsV1alpha1Client) DaemonSets(namespace string) DaemonSetInterface {
	return newDaemonSets(c, namespace)
}

func (c *ComponentsV1alpha1Client) Deployments(namespace string) DeploymentInterface {
	return newDeployments(c, namespace)
}

func (c *ComponentsV1alpha1Client) Ingresses(namespace string) IngressInterface {
	return newIngresses(c, namespace)
}

func (c *ComponentsV1alpha1Client) Secrets(namespace string) SecretInterface {
	return newSecrets(c, namespace)
}

func (c *ComponentsV1alpha1Client) Services(namespace string) ServiceInterface {
	return newServices(c, namespace)
}

func (c *ComponentsV1alpha1Client) ServiceAccounts(namespace string) ServiceAccountInterface {
	return newServiceAccounts(c, namespace)
}

// NewForConfig creates a new ComponentsV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*ComponentsV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ComponentsV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new ComponentsV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *ComponentsV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ComponentsV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *ComponentsV1alpha1Client {
	return &ComponentsV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *ComponentsV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
