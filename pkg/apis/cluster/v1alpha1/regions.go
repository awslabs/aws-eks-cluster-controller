package v1alpha1

// Supported is the current list of supported eks regions 2019-01-15 (https://aws.amazon.com/about-aws/global-infrastructure/regional-product-services/)
var SupportedRegions = []string{
	"us-west-2",
	"us-east-1",
	"us-east-2",
	"eu-central-1",
	"eu-north-1",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-south-1",
	"ap-southeast-1",
	"ap-southeast-2",
}

// IsSupportedRegion reports if the region passed is supported by EKS
func IsSupportedRegion(region *string) bool {
	if region == nil {
		return false
	}
	for _, reg := range SupportedRegions {
		if reg == *region {
			return true
		}
	}
	return false
}
