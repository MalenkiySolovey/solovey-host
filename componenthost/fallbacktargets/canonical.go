package fallbacktargets

// CanonicalTargetV2 validates an existing provider revision and returns its
// canonical representation. It does not invent a new provider revision.
func CanonicalTargetV2(target FallbackTargetV2) (FallbackTargetV2, error) {
	return validatedCanonicalTargetV2(target)
}

// CanonicalReservationV1 validates an existing reservation and normalizes its
// bounded reason codes. It does not reserve or mutate a provider target.
func CanonicalReservationV1(reservation ProviderTargetReservationV1) (ProviderTargetReservationV1, error) {
	return canonicalReservationV1(reservation)
}

// ReasonCodesValidV2 checks the bounded provider contract reason vocabulary.
func ReasonCodesValidV2(values []string) bool {
	return validReasonCodesV2(values)
}
