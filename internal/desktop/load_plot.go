package desktop

import (
	"fmt"
	"math"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"github.com/sya-ri/tovela/internal/relay"
)

const loadAxisInset = 68

type loadPlot struct {
	content  *fyne.Container
	grid     []*canvas.Line
	upper    *canvas.Text
	lower    *canvas.Text
	axes     []fyne.CanvasObject
	capacity float64
	ceiling  float64
	unit     string
	start    time.Time
	samples  []relay.LoadSample
}

func newLoadPlot(capacity float64, unit string) *loadPlot {
	label := func(text string) *canvas.Text {
		value := canvas.NewText(text, theme.Color(theme.ColorNameForeground))
		value.TextSize = theme.Size(theme.SizeNameCaptionText)
		value.Alignment = fyne.TextAlignTrailing
		return value
	}
	plot := &loadPlot{
		capacity: capacity,
		ceiling:  capacity,
		unit:     unit,
		upper:    label(fmt.Sprintf("%.0f%s", capacity, unit)),
		lower:    label("0" + unit),
	}
	for range 3 {
		line := canvas.NewLine(theme.Color(theme.ColorNameSeparator))
		plot.grid = append(plot.grid, line)
		plot.axes = append(plot.axes, line)
	}
	plot.axes = append(plot.axes, plot.upper, plot.lower)
	plot.content = container.New(plot, plot.axes...)
	return plot
}

func (p *loadPlot) update(samples []relay.LoadSample, end time.Time) {
	p.start = end.Add(-relay.LoadWindow)
	p.samples = samples
	p.ceiling = p.capacity
	for _, sample := range samples {
		p.ceiling = max(p.ceiling, sample.Value)
	}
	p.ceiling = math.Ceil(p.ceiling)
	p.upper.Text = fmt.Sprintf("%.0f%s", p.ceiling, p.unit)

	objects := slices.Clone(p.axes)
	color := theme.Color(theme.ColorNamePrimary)
	for i, sample := range samples {
		// Cloud Monitoring samples once a minute. Leave a gap when a sample is missing.
		if i == 0 || sample.At.Sub(samples[i-1].At) > 90*time.Second {
			objects = append(objects, canvas.NewCircle(color))
		} else {
			line := canvas.NewLine(color)
			line.StrokeWidth = 2
			objects = append(objects, line)
		}
	}
	p.content.Objects = objects
	p.content.Refresh()
}

func (p *loadPlot) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	left, right := float32(loadAxisInset), size.Width-8
	top, bottom := float32(10), size.Height-10
	for i, grid := range p.grid {
		y := top + float32(i)*(bottom-top)/2
		grid.Position1, grid.Position2 = fyne.NewPos(left, y), fyne.NewPos(right, y)
		grid.Refresh()
	}
	place := func(label *canvas.Text, y float32) {
		height := label.MinSize().Height
		label.Resize(fyne.NewSize(left-8, height))
		label.Move(fyne.NewPos(0, y-height/2))
	}
	place(p.upper, top)
	place(p.lower, bottom)

	var previous fyne.Position
	for i, object := range objects[len(p.axes):] {
		sample := p.samples[i]
		along := float32(float64(sample.At.Sub(p.start)) / float64(relay.LoadWindow))
		up := float32(sample.Value / p.ceiling)
		point := fyne.NewPos(left+along*(right-left), bottom-up*(bottom-top))
		switch mark := object.(type) {
		case *canvas.Circle:
			mark.Move(point.SubtractXY(2, 2))
			mark.Resize(fyne.NewSquareSize(4))
		case *canvas.Line:
			mark.Position1, mark.Position2 = previous, point
			mark.Refresh()
		}
		previous = point
	}
}

func (*loadPlot) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(176, 66)
}
