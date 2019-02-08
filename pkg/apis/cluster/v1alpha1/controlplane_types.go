package v1alpha1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ControlPlaneSpec defines the desired state of ControlPlane
type ControlPlaneSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ClusterName string `json:"clusterName"`
	// +optional
	Version *string `json:"version,omitempty"`
	// +optional
	Network *Network `json:"network,omitempty"`
}

// ControlPlaneStatus defines the observed state of ControlPlane
type ControlPlaneStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	CertificateAuthorityData string `json:"certificateAuthorityData"`
	Endpoint                 string `json:"endpoint"`
	Status                   string `json:"status"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlane is the Schema for the controlplanes API
// +k8s:openapi-gen=true
type ControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneSpec   `json:"spec,omitempty"`
	Status ControlPlaneStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneList contains a list of ControlPlane
type ControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ControlPlane{}, &ControlPlaneList{})
}

// GetVersion returns the Version of the EKS cluster
func (cp *ControlPlane) GetVersion() string {
	if isSupportedVersion(cp.Spec.Version) {
		return *cp.Spec.Version
	}
	return DefaultVersion
}

// GetNetwork returns validated Network information specified in Spec
func (cp *ControlPlane) GetNetwork() (Network, error) {
	if cp.Spec.Network == nil {
		return DefaultNetwork, nil
	}

	err := validateNetwork(*cp.Spec.Network)
	if err != nil {
		return Network{}, fmt.Errorf("invalid network: %s", err)
	}

	return *cp.Spec.Network, nil
}
