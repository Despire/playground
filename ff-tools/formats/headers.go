package formats

var (
	PdfHeaderStart  = []byte("%PDF")
	ZipHeaderStart  = []byte("PK\x03\x04")
	PngHeaderStart  = []byte("\x89PNG\r\n\x1a\n")
	JpgHeaderStart  = []byte("\xff\xd8\xff")
	WASMHEaderStart = []byte("\x00\x61\x73\x6D\x01\x00\x00\x00")
	NESHEaderStart  = []byte("\x4E\x45\x53\x1A")
	GIFHeaderStart  = []byte("\x47\x49\x46\x38\x39\x61")
	MP3HeaderStart  = []byte("\xFF\xFB")
	ID3V2Header     = []byte("ID3")
)
