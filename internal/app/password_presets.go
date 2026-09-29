package app

import (
	_ "embed"
	"fmt"
	"math"
	"strings"
)

// EFF Short Wordlist #1 (1296 слов, https://www.eff.org/dice), лицензия
// CC BY 3.0 US, © Electronic Frontier Foundation. Своего словаря в репозитории
// нет; этот список короткий (~9 КБ), без омонимов и даёт log2(1296) ≈ 10.3 бит
// на слово.
//
//go:embed eff_short_wordlist_1.txt
var effShortWordlist string

var passphraseWords = strings.Fields(effShortWordlist)

type generatorPreset int

const (
	presetCharacters generatorPreset = iota
	presetPassphrase
	presetPIN
)

const passphraseSeparator = "-"

func (p generatorPreset) name() string {
	switch p {
	case presetPassphrase:
		return "Passphrase (EFF short, 1296 слов)"
	case presetPIN:
		return "PIN (цифры)"
	default:
		return "Символы"
	}
}

// entropyBits оценивает энтропию при равновероятном выборе символов/слов.
// Для символьного режима обязательный символ каждого класса немного снижает
// фактическую энтропию, поэтому значение помечается как приблизительное.
func entropyBits(g passwordGenerator) float64 {
	switch g.preset {
	case presetPassphrase:
		return float64(g.words) * math.Log2(float64(len(passphraseWords)))
	case presetPIN:
		return float64(g.pinLength) * math.Log2(10)
	default:
		alphabet := strings.Join(passwordClasses(g), "")
		if alphabet == "" {
			return 0
		}
		return float64(g.length) * math.Log2(float64(len(alphabet)))
	}
}

func generatePassphrase(words int) (string, error) {
	if words < 1 {
		return "", fmt.Errorf("укажите число слов")
	}
	chosen := make([]string, words)
	for i := range chosen {
		n, err := randomIndex(len(passphraseWords))
		if err != nil {
			return "", err
		}
		chosen[i] = passphraseWords[n]
	}
	return strings.Join(chosen, passphraseSeparator), nil
}

func generatePIN(length int) (string, error) {
	pin := make([]byte, length)
	for i := range pin {
		n, err := randomIndex(10)
		if err != nil {
			return "", err
		}
		pin[i] = byte('0' + n)
	}
	return string(pin), nil
}
