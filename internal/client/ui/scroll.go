package ui

// Scroll moves top by rows, staying between the first row and the last
// screenful of a body of lines in a viewport of height rows.
func Scroll(top, rows, lines, height int) int {
	return min(max(top+rows, 0), max(lines-height, 0))
}
