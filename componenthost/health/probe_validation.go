package health

func HealthContractTokenValidV1(value string, limit int) bool { return healthToken(value, limit) }

// These validators preserve the closed registered-probe contracts. They check
// values only; registration, provider calls and observation stay with the owner.
func ProtocolProbeCapabilityValidV1(value ProtocolProbeCapabilityV1, target ProtocolProbeTargetV1) bool {
	return validProtocolCapability(value, target)
}

func ProtocolProbeRequestValidV1(value ProtocolProbeRequestV1) bool {
	return validProtocolProbeRequest(value)
}

func ValidateProtocolProbeObservationV1(value ProtocolProbeObservationV1, request ProtocolProbeRequestV1, capability ProtocolProbeCapabilityV1) error {
	return validateProtocolObservation(value, request, capability)
}

func LocalProxyProbeCapabilityValidV1(value LocalProxyProbeCapabilityV1, target LocalProxyProbeTargetV1) bool {
	return validLocalProxyProbeCapability(value, target)
}

func LocalProxyProbeRequestValidV1(value LocalProxyProbeRequestV1) bool {
	return validLocalProxyProbeRequest(value)
}

func ValidateLocalProxyProbeObservationV1(value LocalProxyProbeObservationV1, request LocalProxyProbeRequestV1, capability LocalProxyProbeCapabilityV1) error {
	return validateLocalProxyProbeObservation(value, request, capability)
}
