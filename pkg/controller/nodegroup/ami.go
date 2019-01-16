package nodegroup

import (
	"fmt"
)

// The Optimized amis can be found at https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-ami.html

var eksOptimizedAMIs = map[string]string{
	"1.11-us-west-2":      "ami-094fa4044a2a3cf52",
	"1.11-us-east-1":      "ami-0b4eb1d8782fc3aea",
	"1.11-us-east-2":      "ami-053cbe66e0033ebcf",
	"1.11-eu-central-1":   "ami-0ce0ec06e682ee10e",
	"1.11-eu-north-1":     "ami-082e6cf1c07e60241",
	"1.11-eu-west-1":      "ami-0a9006fb385703b54",
	"1.11-ap-northeast-1": "ami-063650732b3e8b38c",
	"1.11-ap-southeast-1": "ami-0549ac6995b998478",
	"1.11-ap-southeast-2": "ami-03297c04f71690a76",

	"1.10-us-west-2":      "ami-07af9511082779ae7",
	"1.10-us-east-1":      "ami-027792c3cc6de7b5b",
	"1.10-us-east-2":      "ami-036130f4127a367f7",
	"1.10-eu-central-1":   "ami-06d069282a5fea248",
	"1.10-eu-north-1":     "ami-04b0f84e5a05e0b30",
	"1.10-eu-west-1":      "ami-03612357ac9da2c7d",
	"1.10-ap-northeast-1": "ami-06f4af3742fca5998",
	"1.10-ap-southeast-1": "ami-0bc97856f0dd86d41",
	"1.10-ap-southeast-2": "ami-05d25b3f16e685c2e",
}

// GetAMI returns the latest AMI for a supported Version and Region. The current ami list can be found at https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-ami.html
func GetAMI(version, region string) string {
	verisonRegion := fmt.Sprintf("%s-%s", version, region)
	return eksOptimizedAMIs[verisonRegion]
}
