package resources

import ()

const // MaxResourceFacts bounds the neutral inventory retained and exposed by a
// single snapshot. A truncated snapshot carries an error so consumers that
// require completeness fail closed.
MaxResourceFacts = 4096

const MaxEndpointsPerResource = 16
