package bencoding

import "errors"

// ErrEoF is returned when all of the input has been processed and no more tokens are left.
var ErrEOF = errors.New("end of file")

// Token represents possible tokens found in a Bencoding.
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
