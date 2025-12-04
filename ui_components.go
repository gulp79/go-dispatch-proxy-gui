package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// MiniGraph è un widget personalizzato per il grafico a linee
type MiniGraph struct {
	widget.BaseWidget
	Data      []float64
	LineColor color.Color
	MaxVal    float64
}

func NewMiniGraph(col color.Color) *MiniGraph {
	m := &MiniGraph{
		Data:      make([]float64, 50),
		LineColor: col,
		MaxVal:    100.0,
	}
	m.ExtendBaseWidget(m)
	return m
}

func (m *MiniGraph) AddValue(v float64) {
	// Shift a sinistra
	copy(m.Data, m.Data[1:])
	m.Data[len(m.Data)-1] = v
	if v > m.MaxVal {
		m.MaxVal = v * 1.2
	} else if m.MaxVal > 100 && v < m.MaxVal*0.5 {
		m.MaxVal = m.MaxVal * 0.9
	}
	m.Refresh()
}

func (m *MiniGraph) CreateRenderer() fyne.WidgetRenderer {
	r := &graphRenderer{m: m}
	r.Init() // ✓ AGGIUNTO: Inizializza il renderer
	return r
}

type graphRenderer struct {
	m    *MiniGraph
	line *canvas.Line
	bg   *canvas.Rectangle
}

func (r *graphRenderer) MinSize() fyne.Size {
	return fyne.NewSize(120, 40) // ✓ Grafico più grande e leggibile
}

func (r *graphRenderer) Layout(s fyne.Size) {}

func (r *graphRenderer) Refresh() {
	if r.bg != nil {
		r.bg.Refresh()
	}
}

func (r *graphRenderer) Objects() []fyne.CanvasObject {
	if r.bg == nil {
		return []fyne.CanvasObject{}
	}
	
	objs := []fyne.CanvasObject{r.bg}
	w := r.m.Size().Width
	h := r.m.Size().Height
	
	// ✓ Controllo dimensioni valide
	if w <= 0 || h <= 0 || len(r.m.Data) < 2 {
		return objs
	}
	
	step := w / float32(len(r.m.Data)-1)

	for i := 0; i < len(r.m.Data)-1; i++ {
		x1 := float32(i) * step
		y1 := h - (float32(r.m.Data[i]) / float32(r.m.MaxVal) * h)
		x2 := float32(i+1) * step
		y2 := h - (float32(r.m.Data[i+1]) / float32(r.m.MaxVal) * h)
		
		line := canvas.NewLine(r.m.LineColor)
		line.StrokeWidth = 2.0 // ✓ Linea più spessa e visibile
		line.Position1 = fyne.NewPos(x1, y1)
		line.Position2 = fyne.NewPos(x2, y2)
		objs = append(objs, line)
	}
	return objs
}

func (r *graphRenderer) Destroy() {}

func (r *graphRenderer) Init() {
	r.bg = canvas.NewRectangle(color.RGBA{30, 30, 30, 255})
}
