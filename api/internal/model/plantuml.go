package model

// PlantUmlRenderReq carries a diagram source to be rendered by the configured server.
type PlantUmlRenderReq struct {
	Source string `json:"source" binding:"required"`
}

// PlantUmlRenderRes carries the rendered SVG.
type PlantUmlRenderRes struct {
	Svg string `json:"svg"`
}
