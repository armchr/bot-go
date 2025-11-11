// convertToInt64 safely converts various integer types to int64
func (cg *CodeGraph) convertToInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case uint64:
		return int64(v)
	case uint32:
		return int64(v)
	case uint:
		return int64(v)
	default:
		cg.logger.Warn("Unexpected type for int64 conversion", zap.Any("value", value), zap.String("type", fmt.Sprintf("%T", value)))
		return 0
	}
}
