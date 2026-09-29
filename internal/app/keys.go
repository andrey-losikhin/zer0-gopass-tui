package app

import tea "github.com/charmbracelet/bubbletea"

// russianLayout maps each key of the Russian ЙЦУКЕН layout to the Latin
// character on the same physical key, so shortcuts work in either layout.
var russianLayout = map[string]string{
	"й": "q",
	"ц": "w",
	"у": "e",
	"к": "r",
	"е": "t",
	"н": "y",
	"г": "u",
	"ш": "i",
	"щ": "o",
	"з": "p",
	"х": "[",
	"ъ": "]",
	"ф": "a",
	"ы": "s",
	"в": "d",
	"а": "f",
	"п": "g",
	"р": "h",
	"о": "j",
	"л": "k",
	"д": "l",
	"ж": ";",
	"э": "'",
	"я": "z",
	"ч": "x",
	"с": "c",
	"м": "v",
	"и": "b",
	"т": "n",
	"ь": "m",
	"б": ",",
	"ю": ".",
	".": "/",
	"Й": "Q",
	"Ц": "W",
	"У": "E",
	"К": "R",
	"Е": "T",
	"Н": "Y",
	"Г": "U",
	"Ш": "I",
	"Щ": "O",
	"З": "P",
	"Х": "{",
	"Ъ": "}",
	"Ф": "A",
	"Ы": "S",
	"В": "D",
	"А": "F",
	"П": "G",
	"Р": "H",
	"О": "J",
	"Л": "K",
	"Д": "L",
	"Ж": ":",
	"Э": "\"",
	"Я": "Z",
	"Ч": "X",
	"С": "C",
	"М": "V",
	"И": "B",
	"Т": "N",
	"Ь": "M",
	"Б": "<",
	"Ю": ">",
	"ё": "`",
	"Ё": "~",
}

func commandKey(msg tea.KeyMsg) string {
	key := msg.String()
	if latin, ok := russianLayout[key]; ok {
		return latin
	}
	return key
}
