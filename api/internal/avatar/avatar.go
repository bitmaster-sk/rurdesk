package avatar

import (
	"math/rand"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// RandomColor generates the default avatar background — a pleasant, evenly
// distributed HCL colour with high-ish luminance. Shared by every user/agent
// creation path so the formula lives in one place.
func RandomColor() string {
	return colorful.Hcl(rand.Float64()*360, rand.Float64(), 0.6+rand.Float64()*0.4).Hex()
}
