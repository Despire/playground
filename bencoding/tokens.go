package bencoding

import "errors"

var ErrEOF = errors.New("end of file")

type Token byte

const (
	valueDelimiter  Token = ':'
	integerBegin    Token = 'i'
	listBegin       Token = 'l'
	dictionaryBegin Token = 'd'
	valueEnd        Token = 'e'
)

func advanceUntil(src []byte, curr int, tok Token) (int, error) {
	for curr < len(src) && src[curr] != byte(tok) {
		curr++
	}
	if curr == len(src) {
		return 0, ErrEOF
	}

	return curr, nil
}
