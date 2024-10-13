package bencoding

type Token byte

const (
	valueDelimiter  Token = ':'
	integerBegin    Token = 'i'
	listBegin       Token = 'l'
	dictionaryBegin Token = 'd'
	valueEnd        Token = 'e'
)
