package dxf

// ACI is an AutoCAD Color Index. Values 1-255 are palette entries; 256 means
// ByLayer and 0 means ByBlock.
type ACI int

// The seven standard colours, which are all most CAM setups distinguish.
const (
	ByBlock ACI = 0
	Red     ACI = 1
	Yellow  ACI = 2
	Green   ACI = 3
	Cyan    ACI = 4
	Blue    ACI = 5
	Magenta ACI = 6
	White   ACI = 7
	Grey    ACI = 8
	ByLayer ACI = 256
)
