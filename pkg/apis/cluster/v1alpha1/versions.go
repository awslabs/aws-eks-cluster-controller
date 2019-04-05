package v1alpha1

// SupportedVersions is the list of current supported verisons of EKS (AC 2019-01-15) (https://docs.aws.amazon.com/eks/latest/userguide/platform-versions.html)
var SupportedVersions = []string{
	"1.11",
	"1.12",
}

// DefaultVersion is the version that is used if none is supplied
const DefaultVersion string = "1.12"

func isSupportedVersion(version *string) bool {
	if version == nil {
		return false
	}
	for _, ver := range SupportedVersions {
		if ver == *version {
			return true
		}
	}
	return false
}
