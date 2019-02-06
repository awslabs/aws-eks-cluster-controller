package v1alpha1

import (
	"errors"
	"testing"
)

func TestValidateNetwork(t *testing.T) {
	tests := []struct {
		name string
		arg  Network
		err  error
	}{
		{
			name: "network with blank fields",
			arg: Network{
				VpcCidr:     "",
				SubnetCidrs: make([]string, 3),
			},
			err: errors.New("invalid CIDR address: "),
		},
		{
			name: "network with one field wrong",
			arg: Network{
				VpcCidr: "172.16.0.0/16",
				SubnetCidrs: []string{
					"test",
					"172.16.1.0/24",
					"172.16.2.0/24",
				},
			},
			err: errors.New("invalid CIDR address: test"),
		},
		{
			name: "network with no ipv4 CIDRs",
			arg: Network{
				VpcCidr: "2001:db8::/100",
				SubnetCidrs: []string{
					"2001:db8::/101",
					"2001:db8::/102",
					"2001:db8::/103",
				},
			},
			err: errors.New("not ipv4 subnet: 2001:db8::/100"),
		},
		{
			name: "network with overlapping CIDRs",
			arg: Network{
				VpcCidr: "172.16.0.0/16",
				SubnetCidrs: []string{
					"172.16.0.0/23",
					"172.16.1.0/24",
					"172.16.2.0/24",
				},
			},
			err: errors.New("172.16.1.0/24 overlaps with 172.16.0.0/23"),
		},
		{
			name: "subet is out of vpc CIDR range",
			arg: Network{
				VpcCidr: "172.16.0.0/16",
				SubnetCidrs: []string{
					"192.168.64.0/18",
					"172.16.1.0/24",
					"172.16.2.0/24",
				},
			},
			err: errors.New("172.16.0.0/16 does not fully contain 192.168.64.0/18"),
		},
		{
			name: "subnets and vpc CIDRs valid",
			arg: Network{
				VpcCidr: "172.16.0.0/16",
				SubnetCidrs: []string{
					"172.16.0.0/24",
					"172.16.1.0/24",
					"172.16.2.0/24",
				},
			},
			err: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNetwork(tc.arg)

			if err != nil && tc.err.Error() != err.Error() {
				t.Errorf("Expected: \n%+v\n\n Got:\n%+v", tc.err, err)
			}
		})
	}

}
