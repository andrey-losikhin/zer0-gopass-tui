package app

import tea "github.com/charmbracelet/bubbletea"

func commandKey(msg tea.KeyMsg) string {
	key := msg.String()
	switch key {
	case "й":
		return "q"
	case "т":
		return "n"
	case "в":
		return "d"
	case "у":
		return "e"
	case "ф":
		return "a"
	case "к":
		return "r"
	case "п":
		return "g"
	case "и":
		return "b"
	case "ч":
		return "x"
	case "о":
		return "j"
	case "л":
		return "k"
	case "р":
		return "h"
	case "д":
		return "l"
	case "н":
		return "y"
	case "м":
		return "v"
	case "ь":
		return "m"
	case ".":
		return "/"
	default:
		return key
	}
}
