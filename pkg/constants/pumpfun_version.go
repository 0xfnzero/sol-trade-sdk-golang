package constants

// PumpFunVersion selects an instruction ABI, independently of message version.
type PumpFunVersion string

const (
	PumpFunLegacy PumpFunVersion = "v1"
	PumpFunV2     PumpFunVersion = "v2"
	PumpFunV3     PumpFunVersion = "v3"
)
