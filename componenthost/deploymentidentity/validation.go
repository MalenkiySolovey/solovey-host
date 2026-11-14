package deploymentidentity

import (
	path "path"
	regexp "regexp"
	strings "strings"
)

var revisionPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

var sourcePattern = regexp.MustCompile(`^src-[a-f0-9]{64}$`)

var artifactPattern = regexp.MustCompile(`^art-[a-f0-9]{64}$`)

var deployPattern = regexp.MustCompile(`^dep-[a-f0-9]{64}$`)

var uuidPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[1-5][a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)

var identityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@+-]{0,127}$`)

func canonicalAbsolute(value string) bool {
	return value != "" && strings.HasPrefix(value, "/") && path.Clean(value) == value && !strings.ContainsRune(value, 0)
}
