package utils

func Min(w, h int) int {
	if w > 0 && h > 0 {
		if w < h {
			return w
		}
		return h
	}
	// 如果只有一个维度有值，返回那个值
	if h > 0 {
		return h
	}
	return w
}
