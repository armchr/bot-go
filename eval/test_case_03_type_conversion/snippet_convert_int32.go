// convertToInt32 safely converts various integer types to int32
func (cg *CodeGraph) convertToInt32(value any) int32 {
	switch v := value.(type) {
	case int32:
		return v
	case int64:
		return int32(v)
	case int:
		return int32(v)
	case uint32:
		return int32(v)
	case uint64:
		return int32(v)
	case uint:
		return int32(v)
	default:
		cg.logger.Warn("Unexpected type for int32 conversion", zap.Any("value", value), zap.String("type", fmt.Sprintf("%T", value)))
		return 0
	}
}
