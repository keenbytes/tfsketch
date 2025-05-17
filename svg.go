package main

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

func generateSvg(module *TfModule) string {
	const (
		rectWidth  = 300
		rectHeight = 50
		vertGap    = 10
		horizGap   = 80
		margin     = 20
		fontSize   = 12
		descFont   = 10
	)

	var inputs, outputs []string
	for name := range module.Variables {
		inputs = append(inputs, name)
	}
	for name := range module.Outputs {
		outputs = append(outputs, name)
	}
	resourceGroups := make(map[string][]string)
	var kinds []string
	for _, res := range module.Resources {
		kind := res.Kind
		if _, ok := resourceGroups[kind]; !ok {
			kinds = append(kinds, kind)
		}
		resourceGroups[kind] = append(resourceGroups[kind], res.Name)
	}
	sort.Strings(inputs)
	sort.Strings(outputs)
	sort.Strings(kinds)

	maxLen := max(len(inputs), len(outputs))
	svgHeight := margin*2 + (rectHeight+vertGap)*maxLen + 40
	svgWidth := margin*2 + rectWidth*3 + horizGap*2

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, svgWidth, svgHeight))

	drawRect := func(x, y int, label, desc string) {
		sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="#d0e6f7" stroke="#333"/>`, x, y, rectWidth, rectHeight))
		labelY := y + rectHeight/2 - 2
		descY := y + rectHeight - 6
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="%d" text-anchor="middle" fill="#000">%s</text>`,
			x+rectWidth/2, labelY, fontSize, htmlEscape(label)))
		if desc != "" {
			sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="%d" text-anchor="middle" fill="#555">%s</text>`,
				x+rectWidth/2, descY, descFont, htmlEscape(desc)))
		}
	}

	inputX := margin
	sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="%d" font-weight="bold" fill="#222">INPUTS</text>`, inputX, margin-5, fontSize+2))
	inputYs := make([]int, len(inputs))
	for i, name := range inputs {
		y := margin + 20 + i*(rectHeight+vertGap)
		inputYs[i] = y
		desc := ""
		if v := module.Variables[name]; v != nil {
			desc = v.Description
		}
		drawRect(inputX, y, name, desc)
	}

	totalResHeight := 0
	for _, kind := range kinds {
		totalResHeight += rectHeight*len(resourceGroups[kind]) + 20
	}
	totalResHeight -= 20

	outerResX := inputX + rectWidth + horizGap
	outerResY := margin + 20
	sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="%d" font-weight="bold" fill="#222">RESOURCES</text>`, outerResX, margin-5, fontSize+2))
	sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="#555" stroke-width="2"/>`, outerResX-10, outerResY-20, rectWidth+20, totalResHeight+40))

	resX := outerResX
	resY := outerResY
	for _, kind := range kinds {
		names := resourceGroups[kind]
		groupHeight := rectHeight * len(names)
		sb.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="#e2f0cb" stroke="#333"/>`, resX, resY, rectWidth, groupHeight))
		for i, name := range names {
			lineY := resY + rectHeight*i + rectHeight/2 + fontSize/3
			sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="%d" text-anchor="middle" alignment-baseline="middle" fill="#000">%s</text>`,
				resX+rectWidth/2, lineY, fontSize, htmlEscape(name)))
		}
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="%d" text-anchor="start" fill="#555">%s</text>`,
			resX, resY-fontSize/2, fontSize, htmlEscape(kind)))
		resY += groupHeight + 20
	}

	outputX := resX + rectWidth + horizGap
	sb.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="%d" font-weight="bold" fill="#222">OUTPUTS</text>`, outputX, margin-5, fontSize+2))
	outputYs := make([]int, len(outputs))
	for i, name := range outputs {
		y := margin + 20 + i*(rectHeight+vertGap)
		outputYs[i] = y
		desc := ""
		if o := module.Outputs[name]; o != nil {
			desc = o.Description
		}
		drawRect(outputX, y, name, desc)
	}

	sb.WriteString("</svg>")
	return sb.String()
}

func htmlEscape(s string) string {
	return html.EscapeString(s)
}
