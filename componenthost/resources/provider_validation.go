package resources

// The existing provider registries share these exact bounded token rules with
// their typed contracts. These predicates grant no registration or lease.
func FrontingContractTokenValidV1(value string, limit int) bool {
	return frontingToken(value, limit)
}

func TransportContractTokenValidV2(value string, limit int) bool {
	return boundedToken(value, limit)
}

func InterceptionContractTokenValidV1(value string, limit int) bool {
	return interceptionToken(value, limit)
}
