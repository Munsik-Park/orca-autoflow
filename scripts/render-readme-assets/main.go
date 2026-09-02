// Package main renders deterministic README image assets.
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

var (
	bg      = color.RGBA{R: 24, G: 24, B: 37, A: 255}
	panel   = color.RGBA{R: 30, G: 30, B: 46, A: 255}
	panel2  = color.RGBA{R: 49, G: 50, B: 68, A: 255}
	text    = color.RGBA{R: 205, G: 214, B: 244, A: 255}
	muted   = color.RGBA{R: 166, G: 173, B: 200, A: 255}
	teal    = color.RGBA{R: 148, G: 226, B: 213, A: 255}
	blue    = color.RGBA{R: 137, G: 180, B: 250, A: 255}
	green   = color.RGBA{R: 166, G: 227, B: 161, A: 255}
	yellow  = color.RGBA{R: 249, G: 226, B: 175, A: 255}
	red     = color.RGBA{R: 243, G: 139, B: 168, A: 255}
	overlay = color.RGBA{R: 88, G: 91, B: 112, A: 255}
	clear   = color.RGBA{}
)

func main() {
	must(os.MkdirAll("assets", 0o755))
	must(writePNG("assets/logo.png", logo()))
	must(writePNG("assets/tui-watch.png", tuiWatch()))
	must(writePNG("assets/tui-review.png", tuiReview()))
	must(writePNG("assets/tui-pod.png", tuiPod()))
}

func logo() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	fill(img, img.Bounds(), clear)

	cx, cy := 256.0, 256.0
	for y := 56; y < 456; y++ {
		for x := 56; x < 456; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > 198 {
				continue
			}
			angle := math.Atan2(dy, dx)
			wave := 14 * math.Sin(angle*3+dist/28)
			if dist > 144+wave && dist < 192 {
				img.Set(x, y, teal)
			} else if dist <= 144+wave {
				img.Set(x, y, bg)
			}
		}
	}

	ellipse(img, 136, 184, 378, 314, color.RGBA{R: 245, G: 245, B: 250, A: 255})
	ellipse(img, 150, 196, 338, 278, color.RGBA{R: 17, G: 17, B: 27, A: 255})
	triangle(img, image.Point{368, 236}, image.Point{456, 184}, image.Point{428, 270}, color.RGBA{R: 245, G: 245, B: 250, A: 255})
	triangle(img, image.Point{244, 188}, image.Point{292, 92}, image.Point{306, 210}, color.RGBA{R: 17, G: 17, B: 27, A: 255})
	ellipse(img, 278, 210, 318, 236, color.RGBA{R: 245, G: 245, B: 250, A: 255})
	ellipse(img, 348, 226, 364, 242, color.RGBA{R: 17, G: 17, B: 27, A: 255})
	arcDots(img, 118, 334, 280, 26, teal)
	arcDots(img, 150, 366, 214, 18, blue)
	drawText(img, 104, 414, "ORCA AUTOFLOW", 4, text)
	return img
}

func tuiWatch() image.Image {
	img := terminalCanvas("WATCH")
	drawText(img, 54, 116, "RUNS", 4, text)
	drawText(img, 594, 116, "PODS", 4, text)
	runRow(img, 54, 166, "RUN A3F3", "RUNNING", green)
	runRow(img, 54, 250, "RUN B821", "READY", blue)
	runRow(img, 54, 334, "RUN C044", "BLOCKED", yellow)
	podBox(img, 594, 166, "BILLING REFACTOR", 0)
	podBox(img, 594, 286, "CLI DOCS", 1)
	statusBar(img, "J/K MOVE   R REVIEW   S SHIP   Q QUIT")
	return img
}

func tuiReview() image.Image {
	img := terminalCanvas("REVIEW")
	drawText(img, 54, 116, "RUN A3F3 DIFF", 4, text)
	drawText(img, 646, 116, "LOGS", 4, text)
	codeLine(img, 54, 166, "+ ADD AUTOFLOW STATE WRITE", green)
	codeLine(img, 54, 214, "+ VERIFY OUTPUT ARTIFACTS", green)
	codeLine(img, 54, 262, "- SKIP CLOSED ISSUE CHECK", red)
	codeLine(img, 54, 310, "+ REQUIRE OPEN ISSUE", green)
	logLine(img, 646, 166, "TESTS PASS", green)
	logLine(img, 646, 214, "STATE SAVED", blue)
	logLine(img, 646, 262, "READY FOR REVIEW", yellow)
	statusBar(img, "ENTER OPEN   S SHIP   K KILL   ESC BACK")
	return img
}

func tuiPod() image.Image {
	img := terminalCanvas("POD")
	drawText(img, 54, 116, "POD DAG", 4, text)
	node(img, 94, 226, "DESIGN", blue)
	node(img, 414, 168, "RED", red)
	node(img, 414, 300, "GREEN", green)
	node(img, 738, 226, "VERIFY", yellow)
	line(img, 274, 266, 414, 206, overlay)
	line(img, 274, 266, 414, 338, overlay)
	line(img, 594, 206, 738, 266, overlay)
	line(img, 594, 338, 738, 266, overlay)
	drawText(img, 88, 472, "DEPENDENCIES CONTROL WHICH RUNS CAN START", 3, muted)
	statusBar(img, "TAB VIEW   / SEARCH   N NEW RUN   Q QUIT")
	return img
}

func terminalCanvas(title string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 720))
	fill(img, img.Bounds(), bg)
	fill(img, image.Rect(26, 26, 1174, 694), panel)
	fill(img, image.Rect(26, 26, 1174, 82), panel2)
	circle(img, 62, 54, 10, red)
	circle(img, 96, 54, 10, yellow)
	circle(img, 130, 54, 10, green)
	drawText(img, 514, 44, "ORCA "+title, 3, text)
	return img
}

func runRow(img *image.RGBA, x, y int, name, status string, accent color.RGBA) {
	fill(img, image.Rect(x, y, x+470, y+58), panel2)
	fill(img, image.Rect(x, y, x+8, y+58), accent)
	drawText(img, x+26, y+18, name, 3, text)
	drawText(img, x+284, y+18, status, 3, accent)
}

func podBox(img *image.RGBA, x, y int, name string, variant int) {
	fill(img, image.Rect(x, y, x+500, y+82), panel2)
	drawText(img, x+24, y+18, name, 3, text)
	colors := []color.RGBA{blue, red, green, yellow}
	for i := 0; i < 4; i++ {
		circle(img, x+42+i*68, y+58, 13, colors[(i+variant)%len(colors)])
		if i > 0 {
			line(img, x+42+(i-1)*68+15, y+58, x+42+i*68-15, y+58, overlay)
		}
	}
}

func codeLine(img *image.RGBA, x, y int, s string, c color.RGBA) {
	fill(img, image.Rect(x, y, x+520, y+34), color.RGBA{R: 17, G: 17, B: 27, A: 255})
	drawText(img, x+14, y+9, s, 2, c)
}

func logLine(img *image.RGBA, x, y int, s string, c color.RGBA) {
	fill(img, image.Rect(x, y, x+420, y+42), panel2)
	fill(img, image.Rect(x, y, x+7, y+42), c)
	drawText(img, x+24, y+12, s, 2, text)
}

func node(img *image.RGBA, x, y int, label string, c color.RGBA) {
	fill(img, image.Rect(x, y, x+180, y+80), panel2)
	fill(img, image.Rect(x, y, x+180, y+8), c)
	drawText(img, x+34, y+34, label, 3, text)
}

func statusBar(img *image.RGBA, s string) {
	fill(img, image.Rect(26, 640, 1174, 694), panel2)
	drawText(img, 54, 660, s, 3, muted)
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	encodeErr := png.Encode(file, img)
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func fill(img draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
}

func ellipse(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	cx, cy := float64(x0+x1)/2, float64(y0+y1)/2
	rx, ry := float64(x1-x0)/2, float64(y1-y0)/2
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			dx, dy := (float64(x)-cx)/rx, (float64(y)-cy)/ry
			if dx*dx+dy*dy <= 1 {
				img.Set(x, y, c)
			}
		}
	}
}

func circle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	ellipse(img, cx-r, cy-r, cx+r, cy+r, c)
}

func triangle(img *image.RGBA, a, b, cpt image.Point, c color.RGBA) {
	minX := min(a.X, min(b.X, cpt.X))
	maxX := max(a.X, max(b.X, cpt.X))
	minY := min(a.Y, min(b.Y, cpt.Y))
	maxY := max(a.Y, max(b.Y, cpt.Y))
	area := edge(a, b, cpt)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			p := image.Point{x, y}
			w0 := edge(b, cpt, p)
			w1 := edge(cpt, a, p)
			w2 := edge(a, b, p)
			if sameSign(area, w0) && sameSign(area, w1) && sameSign(area, w2) {
				img.Set(x, y, c)
			}
		}
	}
}

func edge(a, b, c image.Point) int {
	return (c.X-a.X)*(b.Y-a.Y) - (c.Y-a.Y)*(b.X-a.X)
}

func sameSign(a, b int) bool {
	return (a >= 0 && b >= 0) || (a <= 0 && b <= 0)
}

func line(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := math.Abs(float64(x1 - x0))
	dy := -math.Abs(float64(y1 - y0))
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		circle(img, x0, y0, 3, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func arcDots(img *image.RGBA, x, y, w, step int, c color.RGBA) {
	for i := 0; i < w; i += step {
		yy := y + int(18*math.Sin(float64(i)/34))
		circle(img, x+i, yy, 5, c)
	}
}

func drawText(img *image.RGBA, x, y int, s string, scale int, c color.RGBA) {
	cursor := x
	for _, r := range strings.ToUpper(s) {
		if r == ' ' {
			cursor += 4 * scale
			continue
		}
		glyph, ok := font[r]
		if !ok {
			glyph = font['?']
		}
		for row, bits := range glyph {
			for col, bit := range bits {
				if bit == '1' {
					fill(img, image.Rect(cursor+col*scale, y+row*scale, cursor+(col+1)*scale, y+(row+1)*scale), c)
				}
			}
		}
		cursor += 6 * scale
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

var font = map[rune][]string{
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'C': {"01111", "10000", "10000", "10000", "10000", "10000", "01111"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01111", "10000", "10000", "10111", "10001", "10001", "01111"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'I': {"11111", "00100", "00100", "00100", "00100", "00100", "11111"},
	'J': {"00111", "00010", "00010", "00010", "00010", "10010", "01100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "10101", "01010"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'Z': {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"10010", "10010", "10010", "11111", "00010", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'+': {"00000", "00100", "00100", "11111", "00100", "00100", "00000"},
	'-': {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'/': {"00001", "00001", "00010", "00100", "01000", "10000", "10000"},
	':': {"00000", "00100", "00100", "00000", "00100", "00100", "00000"},
	'?': {"01110", "10001", "00001", "00010", "00100", "00000", "00100"},
}
