package nixy2

// CPU represents the CPU Architecture, be it amd64 or arm64
type GoCPU string

const (
	X86 GoCPU = "amd64"
	ARM GoCPU = "arm64"
)

func (c GoCPU) ColimaARCH() string {
	if c == X86 {
		return "x86_64"
	}
	return "aarch64"
}

func ParseCPUArch(arch string) GoCPU {
	if arch == "arm" || arch == "arm64" || arch == "aarch64" {
		return ARM
	}

	return X86
}
