package tui

const maxContent = 160

type layout struct {
	width   int
	height  int
	offset  int
	content int
	header  bool
	footer  bool
	framed  bool
	body    int
	left    int
	right   int
	gap     int
	stacked bool
	detail  bool
}

func (l layout) text(width int) int {
	if l.framed {
		return max(width-frameWidth, 4)
	}

	return max(width, 4)
}

func (m *Model) layout() layout {
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	l := layout{width: width, height: height, gap: 2}

	pad := 0
	switch {
	case width >= 72:
		pad = 2
	case width >= 40:
		pad = 1
	}

	l.content = min(width-2*pad, clamp(width*4/5, 120, maxContent))
	l.offset = max((width-l.content)/2, 0)

	l.header = height >= 14
	l.footer = height >= 9
	l.body = height - chrome(l)

	if l.body < 4 {
		l.header = false
		l.body = height - chrome(l)
	}
	if l.body < 4 {
		l.footer = false
		l.body = height - chrome(l)
	}
	l.body = max(l.body, 1)

	l.framed = l.content >= 40 && l.body >= 4
	l.stacked = l.content < 76
	l.detail = !l.stacked

	l.left, l.right = l.content, l.content
	if l.detail {
		l.left = l.content * 55 / 100
		l.right = l.content - l.left - l.gap
	}

	return l
}

func chrome(l layout) int {
	rows := 0
	if l.header {
		rows += 3
	}
	if l.footer {
		rows += 3
	}

	return rows
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
