package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRoleBinding is the Schema for the serviceaccounts API
// +k8s:openapi-gen=true
type ClusterRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRoleBindingSpec   `json:"spec,omitempty"`
	Status ClusterRoleBindingStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRoleBindingList contains a list of ClusterRoleBinding
type ClusterRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRoleBinding `json:"items"`
}

// ClusterRoleBindingSpec defines the desired state of Secret
type ClusterRoleBindingSpec struct {
	rbacv1.ClusterRoleBinding `json:",inline"`
	Cluster                   string `json:"cluster"`
	Name                      string `json:"name"`
	// ClusterRole isn't a namespaced resource - we need this here so that we can build/run tests
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ClusterRoleBindingStatus defines the observed state of ClusterRoleBinding
type ClusterRoleBindingStatus struct {
	Status string `json:"status,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ClusterRoleBinding{}, &ClusterRoleBindingList{})
}
