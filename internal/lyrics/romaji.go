package lyrics

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

type Romanizer struct {
	tokenizer *tokenizer.Tokenizer
}

func NewRomanizer() (*Romanizer, error) {
	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return nil, fmt.Errorf("lyrics: initialize Japanese tokenizer: %w", err)
	}
	return &Romanizer{tokenizer: t}, nil
}

func (r *Romanizer) Romanize(text string) string {
	var result strings.Builder
	tokens := r.tokenizer.Tokenize(text)
	for i, token := range tokens {
		if i > 0 && needsRomajiSpace(tokens[i-1].Surface, token.Surface) {
			result.WriteByte(' ')
		}
		reading, ok := token.Pronunciation()
		if !ok || reading == "" || reading == "*" {
			reading = token.Surface
		}
		result.WriteString(kanaToRomaji(reading))
	}
	return result.String()
}

func needsRomajiSpace(previous, current string) bool {
	previousRunes, currentRunes := []rune(previous), []rune(current)
	if len(previousRunes) == 0 || len(currentRunes) == 0 {
		return false
	}
	previousLast, currentFirst := previousRunes[len(previousRunes)-1], currentRunes[0]
	return !unicode.IsSpace(previousLast) &&
		!unicode.IsSpace(currentFirst) &&
		!unicode.IsPunct(previousLast) &&
		!unicode.IsPunct(currentFirst)
}

var kanaRomaji = map[rune]string{
	'あ': "a", 'い': "i", 'う': "u", 'え': "e", 'お': "o",
	'か': "ka", 'き': "ki", 'く': "ku", 'け': "ke", 'こ': "ko",
	'が': "ga", 'ぎ': "gi", 'ぐ': "gu", 'げ': "ge", 'ご': "go",
	'さ': "sa", 'し': "shi", 'す': "su", 'せ': "se", 'そ': "so",
	'ざ': "za", 'じ': "ji", 'ず': "zu", 'ぜ': "ze", 'ぞ': "zo",
	'た': "ta", 'ち': "chi", 'つ': "tsu", 'て': "te", 'と': "to",
	'だ': "da", 'ぢ': "ji", 'づ': "zu", 'で': "de", 'ど': "do",
	'な': "na", 'に': "ni", 'ぬ': "nu", 'ね': "ne", 'の': "no",
	'は': "ha", 'ひ': "hi", 'ふ': "fu", 'へ': "he", 'ほ': "ho",
	'ば': "ba", 'び': "bi", 'ぶ': "bu", 'べ': "be", 'ぼ': "bo",
	'ぱ': "pa", 'ぴ': "pi", 'ぷ': "pu", 'ぺ': "pe", 'ぽ': "po",
	'ま': "ma", 'み': "mi", 'む': "mu", 'め': "me", 'も': "mo",
	'や': "ya", 'ゆ': "yu", 'よ': "yo",
	'ら': "ra", 'り': "ri", 'る': "ru", 'れ': "re", 'ろ': "ro",
	'わ': "wa", 'ゐ': "wi", 'ゑ': "we", 'を': "o", 'ん': "n",
	'ゔ': "vu", 'ぁ': "a", 'ぃ': "i", 'ぅ': "u", 'ぇ': "e", 'ぉ': "o",
	'ゃ': "ya", 'ゅ': "yu", 'ょ': "yo", 'ゎ': "wa",
	'ゕ': "ka", 'ゖ': "ke",
}

var kanaDigraphRomaji = map[[2]rune]string{
	{'き', 'ゃ'}: "kya", {'き', 'ゅ'}: "kyu", {'き', 'ょ'}: "kyo",
	{'ぎ', 'ゃ'}: "gya", {'ぎ', 'ゅ'}: "gyu", {'ぎ', 'ょ'}: "gyo",
	{'し', 'ゃ'}: "sha", {'し', 'ゅ'}: "shu", {'し', 'ょ'}: "sho", {'し', 'ぇ'}: "she",
	{'じ', 'ゃ'}: "ja", {'じ', 'ゅ'}: "ju", {'じ', 'ょ'}: "jo", {'じ', 'ぇ'}: "je",
	{'ち', 'ゃ'}: "cha", {'ち', 'ゅ'}: "chu", {'ち', 'ょ'}: "cho", {'ち', 'ぇ'}: "che",
	{'に', 'ゃ'}: "nya", {'に', 'ゅ'}: "nyu", {'に', 'ょ'}: "nyo",
	{'ひ', 'ゃ'}: "hya", {'ひ', 'ゅ'}: "hyu", {'ひ', 'ょ'}: "hyo",
	{'び', 'ゃ'}: "bya", {'び', 'ゅ'}: "byu", {'び', 'ょ'}: "byo",
	{'ぴ', 'ゃ'}: "pya", {'ぴ', 'ゅ'}: "pyu", {'ぴ', 'ょ'}: "pyo",
	{'み', 'ゃ'}: "mya", {'み', 'ゅ'}: "myu", {'み', 'ょ'}: "myo",
	{'り', 'ゃ'}: "rya", {'り', 'ゅ'}: "ryu", {'り', 'ょ'}: "ryo",
	{'う', 'ぃ'}: "wi", {'う', 'ぇ'}: "we", {'う', 'ぉ'}: "wo",
	{'く', 'ぁ'}: "kwa", {'く', 'ぃ'}: "kwi", {'く', 'ぇ'}: "kwe", {'く', 'ぉ'}: "kwo",
	{'て', 'ぃ'}: "ti", {'て', 'ゅ'}: "tyu", {'で', 'ぃ'}: "di", {'で', 'ゅ'}: "dyu",
	{'と', 'ぅ'}: "tu", {'ど', 'ぅ'}: "du",
	{'ふ', 'ぁ'}: "fa", {'ふ', 'ぃ'}: "fi", {'ふ', 'ぇ'}: "fe", {'ふ', 'ぉ'}: "fo",
	{'つ', 'ぁ'}: "tsa", {'つ', 'ぃ'}: "tsi", {'つ', 'ぇ'}: "tse", {'つ', 'ぉ'}: "tso",
	{'ゔ', 'ぁ'}: "va", {'ゔ', 'ぃ'}: "vi", {'ゔ', 'ぇ'}: "ve", {'ゔ', 'ぉ'}: "vo",
}

func kanaToRomaji(text string) string {
	runes := []rune(text)
	var result strings.Builder
	geminate := false

	for i := 0; i < len(runes); i++ {
		current := hiragana(runes[i])
		if current == 'っ' {
			geminate = true
			continue
		}
		if current == 'ー' {
			if vowel := lastVowel(result.String()); vowel != 0 {
				result.WriteRune(vowel)
			}
			continue
		}

		mora := kanaRomaji[current]
		if i+1 < len(runes) {
			if digraph, ok := kanaDigraphRomaji[[2]rune{current, hiragana(runes[i+1])}]; ok {
				mora = digraph
				i++
			}
		}
		if mora == "" {
			result.WriteRune(runes[i])
			geminate = false
			continue
		}

		if geminate && len(mora) > 0 && !strings.ContainsRune("aeioun", rune(mora[0])) {
			result.WriteByte(mora[0])
		}
		geminate = false

		if current == 'ん' && i+1 < len(runes) {
			next := hiragana(runes[i+1])
			if strings.ContainsRune("あいうえおやゆよ", next) {
				mora = "n'"
			}
		}
		result.WriteString(mora)
	}
	return result.String()
}

func hiragana(r rune) rune {
	if r >= 'ァ' && r <= 'ヶ' {
		return r - ('ァ' - 'ぁ')
	}
	return r
}

func lastVowel(text string) rune {
	runes := []rune(text)
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]
		if strings.ContainsRune("aeiou", unicode.ToLower(r)) {
			return unicode.ToLower(r)
		}
	}
	return 0
}
