package embedding

import "strconv"

func parseSparseWeights(raw map[string]float32) SparseVector {
	if len(raw) == 0 {
		return nil
	}
	out := make(SparseVector, len(raw))
	for key, value := range raw {
		if value == 0 {
			continue
		}
		id, err := strconv.ParseUint(key, 10, 32)
		if err != nil {
			continue
		}
		out[uint32(id)] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
