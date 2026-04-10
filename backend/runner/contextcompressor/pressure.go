package contextcompressor

// PressureLevel 压力等级
type PressureLevel int

const (
	// PressureLow 低压力 - 不需要压缩
	PressureLow PressureLevel = iota
	// PressureMedium 中压力 - 可以考虑微压缩
	PressureMedium
	// PressureHigh 高压力 - 需要压缩
	PressureHigh
	// PressureCritical 临界压力 - 必须压缩，阻塞
	PressureCritical
)

// EvaluatePressure 根据 token 使用比例评估压力等级
func EvaluatePressure(tokenCount, threshold int) PressureLevel {
	if threshold <= 0 {
		return PressureLow
	}

	ratio := float64(tokenCount) / float64(threshold)
	switch {
	case ratio < 0.7:
		return PressureLow
	case ratio < 0.85:
		return PressureMedium
	case ratio < 0.95:
		return PressureHigh
	default:
		return PressureCritical
	}
}

// String 返回压力等级的字符串表示
func (p PressureLevel) String() string {
	switch p {
	case PressureLow:
		return "low"
	case PressureMedium:
		return "medium"
	case PressureHigh:
		return "high"
	case PressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}