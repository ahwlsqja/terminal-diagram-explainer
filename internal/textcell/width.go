package textcell

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Width returns a deterministic terminal-cell width for the supported label set.
// Ambiguous-width runes count as one cell; Hangul, CJK, Kana and fullwidth forms
// count as two. Control and bidi-formatting runes are rejected before rendering.
func Width(text string) (int, error) {
	if !utf8.ValidString(text) {
		return 0, fmt.Errorf("유효하지 않은 UTF-8")
	}

	width := 0
	hasBase := false
	combiningCount := 0
	for _, r := range text {
		if isForbidden(r) {
			return 0, fmt.Errorf("터미널 제어 문자 U+%04X는 사용할 수 없음", r)
		}
		if unicode.In(r, unicode.Mn, unicode.Me) {
			if !hasBase {
				return 0, fmt.Errorf("결합 문자는 label 시작에 올 수 없음")
			}
			combiningCount++
			if combiningCount > 8 {
				return 0, fmt.Errorf("한 글자에 결합 문자를 8개 넘게 사용할 수 없음")
			}
			continue
		}
		hasBase = true
		combiningCount = 0
		if isWide(r) {
			width += 2
		} else {
			width++
		}
	}
	return width, nil
}

func RuneWidth(r rune) (int, error) {
	if isForbidden(r) {
		return 0, fmt.Errorf("터미널 제어 문자 U+%04X는 사용할 수 없음", r)
	}
	if unicode.In(r, unicode.Mn, unicode.Me) {
		return 0, nil
	}
	if isWide(r) {
		return 2, nil
	}
	return 1, nil
}

func isForbidden(r rune) bool {
	if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
		return true
	}
	switch {
	case r == '\u200e' || r == '\u200f':
		return true
	case r >= '\u202a' && r <= '\u202e':
		return true
	case r >= '\u2066' && r <= '\u2069':
		return true
	case r == '\u200d' || r == '\ufe0e' || r == '\ufe0f':
		return true
	default:
		return false
	}
}

func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115f:
		return true
	case r >= 0x2329 && r <= 0x232a:
		return true
	case r >= 0x2e80 && r <= 0xa4cf:
		return true
	case r >= 0xac00 && r <= 0xd7a3:
		return true
	case r >= 0xf900 && r <= 0xfaff:
		return true
	case r >= 0xfe10 && r <= 0xfe19:
		return true
	case r >= 0xfe30 && r <= 0xfe6f:
		return true
	case r >= 0xff01 && r <= 0xff60:
		return true
	case r >= 0xffe0 && r <= 0xffe6:
		return true
	case r >= 0x1f300 && r <= 0x1faff:
		return true
	case r >= 0x20000 && r <= 0x3fffd:
		return true
	default:
		return false
	}
}
