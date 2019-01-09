package v1alpha1

var ValidRegions = []string{
	"us-west-2",
	"us-east-1",
	"us-east-2",
	"eu-central-1",
	"eu-north-1",
	"eu-west-1",
	"ap-northeast-1",
	"ap-southeast-1",
	"ap-southeast-2",
}

func IsValidRegion(region *string) bool {
	if region == nil {
		return false
	}
	for _, reg := range ValidRegions {
		if reg == *region {
			return true
		}
	}
	return false
}
