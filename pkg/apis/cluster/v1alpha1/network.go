package v1alpha1

import (
	"fmt"
	gocidr "github.com/apparentlymart/go-cidr/cidr"
	"net"
)

// Network encapsulates VPC, Subnet1, Subnet2, and Subnet3 CIDRs for EKS cluster
type Network struct {
	VpcCidr string `json:"vpcCidr"`
	// +kubebuilder:validation:MaxItems=3
	// +kubebuilder:validation:MinItems=3
	SubnetCidrs []string `json:"subnetCidrs"`
}

// DefaultNetwork consists default subnet CIDRs for EKS cluster
var DefaultNetwork = Network{
	VpcCidr: "192.168.0.0/16",
	SubnetCidrs: []string{
		"192.168.64.0/18",
		"192.168.128.0/18",
		"192.168.192.0/18",
	},
}

func validateNetwork(network Network) error {
	cidrs := []string{network.VpcCidr}
	for _, subnet := range network.SubnetCidrs {
		cidrs = append(cidrs, subnet)
	}

	vpcCidr := &net.IPNet{}
	subnets := []*net.IPNet{}

	for i, cidr := range cidrs {
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}

		if ip.To4() == nil {
			return fmt.Errorf("not ipv4 subnet: %s", cidr)
		}

		if i == 0 {
			vpcCidr = ipNet
		} else {
			subnets = append(subnets, ipNet)
		}
	}

	if err := gocidr.VerifyNoOverlap(subnets, vpcCidr); err != nil {
		return err
	}

	return nil
}
