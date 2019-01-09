package v1alpha1

var ValidVersions = []string{
	"1.10",
	"1.11",
}

const DefaultVersion string = "1.11"

func isValidVersion(version *string) bool {
	if version == nil {
		return false
	}
	for _, ver := range ValidVersions {
		if ver == *version {
			return true
		}
	}
	return false
}
