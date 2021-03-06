// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"

	"golang.org/x/perf/v2/benchproc"
	"golang.org/x/perf/v2/benchstat"
	"golang.org/x/perf/v2/benchunit"
)

// A Stack is a Cell that visualizes the cumulative sum of some phase metric.
//
// Each individual Stack has an independent sequence of phases, but
// the changes within a phase are visualized across the row.
type Stack struct {
	row       *stackRow
	unitClass benchunit.UnitClass

	phases OMap // phase config -> stackPhase

	sum float64
}

type stackPhase struct {
	start, end float64
}

func (p stackPhase) len() float64 {
	return p.end - p.start
}

type stackRow struct {
	phaseOrder []*benchproc.Config
	topPhases  map[*benchproc.Config]bool
}

func NewStacks(dists []*OMap, unitClass benchunit.UnitClass) []Cell {
	// Collect phases and create cells.
	row := &stackRow{}
	cells := make([]Cell, len(dists))
	phaseMaxes := make(map[*benchproc.Config]float64)
	var maxSum float64
	var phaseOrders [][]*benchproc.Config
	for i, phases := range dists {
		stack := &Stack{
			row:       row,
			unitClass: unitClass,
		}
		// Accumulate phases.
		var csum float64
		for _, phaseCfg := range phases.Keys {
			dist := phases.Load(phaseCfg).(*benchstat.Distribution)
			stack.phases.Store(phaseCfg, stackPhase{csum, csum + dist.Center})
			csum += dist.Center

			if dist.Center > phaseMaxes[phaseCfg] {
				phaseMaxes[phaseCfg] = dist.Center
			}
		}
		stack.sum = csum
		if csum > maxSum {
			maxSum = csum
		}
		phaseOrders = append(phaseOrders, phases.Keys)

		cells[i] = stack
	}

	// Construct a global phase order.
	row.phaseOrder = globalOrder(phaseOrders)

	// Compute top N phases > 1%.
	const maxTopPhases = 15
	const minPhaseFrac = 0.01
	var topPhases []*benchproc.Config
	for cfg, max := range phaseMaxes {
		if max >= maxSum*minPhaseFrac {
			topPhases = append(topPhases, cfg)
		}
	}
	// Get the top N phases.
	sort.Slice(topPhases, func(i, j int) bool {
		p1 := phaseMaxes[topPhases[i]]
		p2 := phaseMaxes[topPhases[j]]
		return p1 > p2
	})
	if len(topPhases) > maxTopPhases {
		topPhases = topPhases[:maxTopPhases]
	}
	// Put back into a map.
	row.topPhases = make(map[*benchproc.Config]bool)
	for _, cfg := range topPhases {
		row.topPhases[cfg] = true
	}

	return cells
}

func (s *Stack) Extents(ext *Extents) {
	expandScale(&ext.X, 0, 1)
	expandScale(&ext.Y, 0, s.sum)
	// Leave room for the "total" at the bottom.
	ext.Margins.Bottom = labelFontHeight
	// Add phases for phase coloring.
	var prevTop, prevOther *benchproc.Config
	for _, phaseCfg := range s.phases.Keys {
		if s.row.topPhases[phaseCfg] {
			ext.TopPhases.Add(prevTop, phaseCfg)
			// This top phase separates "other" phases, so
			// set prevOther to nil.
			prevTop, prevOther = phaseCfg, nil
		} else {
			ext.OtherPhases.Add(prevOther, phaseCfg)
			prevOther = phaseCfg
		}
	}
}

func (s *Stack) Render(svg *SVG, scales *Scales, prev Cell, prevRight float64) {
	x, y := scales.X, scales.Y
	for _, phaseCfg := range s.phases.Keys {
		phase := s.phases.Load(phaseCfg).(stackPhase)
		fill := svgColor(scales.Colors[phaseCfg])
		title := phaseCfg.Val()

		// Draw rectangle for this phase.
		path := svgPathRect(x.Map(0), y.Map(phase.start), x.Map(1), y.Map(phase.end))
		fmt.Fprintf(svg, `  <path d="%s" fill="%s"><title>%s (%s)</title></path>`+"\n", path, fill, title, benchunit.Scale(phase.len(), s.unitClass))

		// Phase label.
		clipID := svg.GenID("clip")
		fmt.Fprintf(svg, `  <clipPath id="%s"><path d="%s" /></clipPath>`+"\n", clipID, path)
		fmt.Fprintf(svg, `  <text x="%f" y="%f" clip-path="url(#%s)" font-size="%d" text-anchor="middle" dy=".4em">%s (%.0f%%)</text>`+"\n", x.Map(0.5), (y.Map(phase.start)+y.Map(phase.end))/2, clipID, labelFontSize, benchunit.Scale(phase.len(), s.unitClass), 100*phase.len()/s.sum)

		// Connect to phase in previous column.
		if prev, ok := prev.(*Stack); ok {
			phase0, ok := prev.phases.Load(phaseCfg).(stackPhase)
			if !ok {
				continue
			}
			path := fmt.Sprintf("M%f %fL%f %fV%fL%f %fz", prevRight, y.Map(phase0.start), x.Map(0), y.Map(phase.start), y.Map(phase.end), prevRight, y.Map(phase0.end))
			fmt.Fprintf(svg, `  <path d="%s" fill="%s" fill-opacity="0.5" />`+"\n", path, fill)
			// Delta label.
			clipID := svg.GenID("clip")
			fmt.Fprintf(svg, `  <clipPath id="%s"><path d="%s" /></clipPath>`+"\n", clipID, path)
			x := mid(prevRight, scales.Outer.Left)
			y := (y.Map(phase0.start) + y.Map(phase0.end) + y.Map(phase.start) + y.Map(phase.end)) / 4
			fmt.Fprintf(svg, `  <text x="%f" y="%f" clip-path="url(#%s)" font-size="%d" text-anchor="middle" dy=".4em">%+.0f%%</text>`+"\n", x, y, clipID, labelFontSize, 100*(phase.len()/phase0.len()-1))
		}
	}

	// Total
	label := benchunit.Scale(s.sum, s.unitClass)
	totalY := scales.Outer.Bottom - labelFontHeight + labelFontSize
	fmt.Fprintf(svg, `  <text x="%f" y="%f" font-size="%d" text-anchor="middle">%s</text>`+"\n", x.Map(0.5), totalY, labelFontSize, label)
	if prev, ok := prev.(*Stack); ok {
		fmt.Fprintf(svg, `  <text x="%f" y="%f" font-size="%d" text-anchor="middle">%+.0f%%</text>`+"\n", mid(prevRight, scales.Outer.Left), totalY, labelFontSize, 100*(s.sum/prev.sum-1))
	}
}

func (s *Stack) RenderKey(svg *SVG, x float64, lastScales *Scales) (right, bot float64) {
	y := lastScales.Y
	lastRight := lastScales.Outer.Right

	// Create initial visual intervals. The last cell may not have
	// all phases, so we follow the global phase order and figure
	// out where missing phases would go.
	var intervals []interval
	var phase stackPhase
	for _, phaseCfg := range s.row.phaseOrder {
		if phaseX, ok := s.phases.LoadOK(phaseCfg); ok {
			phase = phaseX.(stackPhase)
		} else {
			phase.start = phase.end
		}
		if s.row.topPhases[phaseCfg] {
			mid := (y.Map(phase.start) + y.Map(phase.end)) / 2
			in := interval{mid - keyFontHeight/2, mid + keyFontHeight/2, phaseCfg}
			intervals = append(intervals, in)
		}
	}

	// Slide intervals to remove overlaps.
	removeIntervalOverlaps(intervals)
	// Emit labels
	for _, in := range intervals {
		phaseCfg := in.data.(*benchproc.Config)
		if phaseX, ok := s.phases.LoadOK(phaseCfg); ok {
			phase = phaseX.(stackPhase)
		} else {
			phase.start = phase.end
		}
		label := phaseCfg.Val()
		stroke := svgColor(lastScales.Colors[phaseCfg])
		fmt.Fprintf(svg, `  <text x="%f" y="%f" font-size="%d" dominant-baseline="central">%s</text>`+"\n", x+keyFontSize/2, in.mid(), keyFontSize, label)
		fmt.Fprintf(svg, `  <path d="%s" stroke="%s" stroke-width="2px" fill="none" />`+"\n",
			svgPathHSquiggle(
				lastRight, mid(y.Map(phase.start), y.Map(phase.end)),
				x, in.mid(),
			),
			stroke)
		if in.end > bot {
			bot = in.end
		}
	}
	right = x + keyWidth

	return
}
