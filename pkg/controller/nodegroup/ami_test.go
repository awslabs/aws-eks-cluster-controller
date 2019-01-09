package nodegroup

import (
	"fmt"
	"testing"

	clusterv1alpha1 "github.com/awslabs/aws-eks-cluster-controller/pkg/apis/cluster/v1alpha1"
)

func TestGetAMI(t *testing.T) {
	type test struct {
		name    string
		version string
		region  string
	}

	tests := []test{}
	for _, version := range clusterv1alpha1.ValidVersions {
		for _, region := range clusterv1alpha1.ValidRegions {
			tests = append(tests, test{
				name:    fmt.Sprintf("There should be a valid AMI for v%s in %s", version, region),
				version: version,
				region:  region,
			})
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAMI(tt.version, tt.region); got == "" {
				t.Errorf("GetAMI() return nothing, want AMI-*")
			}
		})
	}
}
