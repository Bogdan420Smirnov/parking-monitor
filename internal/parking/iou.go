package parking

import "math"

func calculateIOU(rect [4]int, box [4]float32) float64 {

	r := [4]float32{float32(rect[0]), float32(rect[1]), float32(rect[2]), float32(rect[3])}

	x1 := float32(math.Max(float64(r[0]), float64(box[0])))
	y1 := float32(math.Max(float64(r[1]), float64(box[1])))
	x2 := float32(math.Min(float64(r[2]), float64(box[2])))
	y2 := float32(math.Min(float64(r[3]), float64(box[3])))

	if x2 < x1 || y2 < y1 {
		return 0.0
	}

	interArea := (x2 - x1) * (y2 - y1)
	rectArea := (r[2] - r[0]) * (r[3] - r[1])
	boxArea := (box[2] - box[0]) * (box[3] - box[1])
	unionArea := rectArea + boxArea - interArea
	if unionArea <= 0 {
		return 0.0
	}
	return float64(interArea / unionArea)
}
