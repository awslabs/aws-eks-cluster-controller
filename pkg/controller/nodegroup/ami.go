package nodegroup

import (
	"fmt"
)

// The Optimized amis can be found at https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-ami.html

var eksOptimizedAMIs = map[string]string{

	"1.12-us-west-2":      "ami-0923e4b35a30a5f53",
	"1.12-us-east-1":      "ami-0abcb9f9190e867ab",
	"1.12-us-east-2":      "ami-04ea7cb66af82ae4a",
	"1.12-eu-central-1":   "ami-0d741ed58ca5b342e",
	"1.12-eu-north-1":     "ami-0c65a309fc58f6907",
	"1.12-eu-west-1":      "ami-08716b70cac884aaa",
	"1.12-eu-west-2":      "ami-0c7388116d474ee10",
	"1.12-eu-west-3":      "ami-0560aea042fec8b12",
	"1.12-ap-northeast-1": "ami-0bfedee6a7845c26d",
	"1.12-ap-northeast-2": "ami-0a904348b703e620c",
	"1.12-ap-south-1":     "ami-09c3eb35bb3be46a4",
	"1.12-ap-southeast-1": "ami-07b922b9b94d9a6d2",
	"1.12-ap-southeast-2": "ami-0f0121e9e64ebd3dc",

	"1.11-us-west-2":      "ami-05ecac759c81e0b0c",
	"1.11-us-east-1":      "ami-02c1de421df89c58d",
	"1.11-us-east-2":      "ami-03b1b6cc34c010f9c",
	"1.11-eu-central-1":   "ami-0c2709025eb548246",
	"1.11-eu-north-1":     "ami-084bd3569d08c6e67",
	"1.11-eu-west-1":      "ami-0e82e73403dd69fa3",
	"1.11-eu-west-2":      "ami-0da9aa88dd2ec8297",
	"1.11-eu-west-3":      "ami-099369bc73d1cc66f",
	"1.11-ap-northeast-1": "ami-0d555d5f56c843803",
	"1.11-ap-northeast-2": "ami-0144ae839b1111571",
	"1.11-ap-south-1":     "ami-02071c0110dc365ba",
	"1.11-ap-southeast-1": "ami-00c91afdb73cf7f93",
	"1.11-ap-southeast-2": "ami-05f4510fcfe56961c",
}

// GetAMI returns the latest AMI for a supported Version and Region. The current ami list can be found at https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-ami.html
func GetAMI(version, region string) string {
	verisonRegion := fmt.Sprintf("%s-%s", version, region)
	return eksOptimizedAMIs[verisonRegion]
}
