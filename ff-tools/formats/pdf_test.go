package formats

import (
	"reflect"
	"testing"
)

func TestPdf_Unwrap(t *testing.T) {
	type fields struct {
		contents []byte
	}
	type args struct {
		b   []byte
		beg []byte
		end []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
	}{
		{
			name: "t-01",
			args: args{
				b:   []byte("/ObjectContents[aabbbb]"),
				beg: []byte("/ObjectContents["),
				end: []byte("]"),
			},
			want: []byte("aabbbb"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pdf{
				contents: tt.fields.contents,
			}
			if got := p.unwrap(tt.args.b, tt.args.beg, tt.args.end); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unwrap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPdf_UnwrapObject(t *testing.T) {
	type fields struct {
		contents []byte
	}
	type args struct {
		b     []byte
		start []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
	}{
		{
			name: "t-2",
			args: args{
				b:     []byte("/I6 6 0 R /Font /Type /Helvetica /I7 7 0 R"),
				start: []byte("/I7"),
			},
			want: []byte("/I7 7 0 R"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pdf{
				contents: tt.fields.contents,
			}
			if got := p.unwrapObject(tt.args.b, tt.args.start); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unwrapObject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPdf_Sanitize(t *testing.T) {
	type fields struct {
		contents []byte
	}
	type args struct {
		b []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
	}{
		{
			name: "t-3",
			args: args{
				b: []byte(`%PDF-1.3
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [ 3 0 R ] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Arial >> >> >> /Contents 4 0 R >>
endobj
4 0 obj
<< /Length 44 >>
stream
BT
/F1 100 Tf
10 400 Td
(Hello World!) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000010 00000 n 
0000000060 00000 n 
0000000120 00000 n 
0000000269 00000 n 
trailer
<< /Root 1 0 R /Size 5 >>
startxref
364
%%EOF`),
			},
			want: []byte(`%PDF-1.3
1 0 obj
<<
 /Type /Catalog /Pages 2 0 R 
>>
endobj
2 0 obj
<<
 /Type /Pages /Kids [ 3 0 R ] /Count 1 
>>
endobj
3 0 obj
<<
 /Type /Page /Parent 2 0 R /Resources 
<<
 /Font 
<<
 /F1 
<<
 /Type /Font /Subtype /Type1 /BaseFont /Arial 
>>
>>
>>
 /Contents 4 0 R 
>>
endobj
4 0 obj
<<
 /Length 44 
>>
stream
BT
/F1 100 Tf
10 400 Td
(Hello World!) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000010 00000 n 
0000000060 00000 n 
0000000120 00000 n 
0000000269 00000 n 
trailer
<<
 /Root 1 0 R /Size 5 
>>
startxref
364
%%EOF
`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pdf{
				contents: tt.fields.contents,
			}
			if got := p.sanitize(tt.args.b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sanitize() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestPdf_FixXref(t *testing.T) {
	type fields struct {
		contents []byte
	}
	type args struct {
		b []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
	}{
		{
			name: "t-4",
			args: args{
				b: []byte(`%PDF-1.3
1 0 obj
<<
 /Type /Catalog /Pages 2 0 R 
>>
endobj
2 0 obj
<<
 /Type /Pages /Kids [ 3 0 R ] /Count 1 
>>
endobj
3 0 obj
<<
 /Type /Page /Parent 2 0 R /Resources 
<<
 /Font 
<<
 /F1 
<<
 /Type /Font /Subtype /Type1 /BaseFont /Arial 
>>
>>
>>
 /Contents 4 0 R 
>>
endobj
4 0 obj
<<
 /Length 44 
>>
stream
BT
/F1 100 Tf
10 400 Td
(Hello World!) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000010 00000 n 
0000000060 00000 n 
0000000120 00000 n 
0000000269 00000 n 
trailer
<<
 /Root 1 0 R /Size 5 
>>
startxref
364
%%EOF`),
			},
			want: []byte(`%PDF-1.3
1 0 obj
<<
 /Type /Catalog /Pages 2 0 R 
>>
endobj
2 0 obj
<<
 /Type /Pages /Kids [ 3 0 R ] /Count 1 
>>
endobj
3 0 obj
<<
 /Type /Page /Parent 2 0 R /Resources 
<<
 /Font 
<<
 /F1 
<<
 /Type /Font /Subtype /Type1 /BaseFont /Arial 
>>
>>
>>
 /Contents 4 0 R 
>>
endobj
4 0 obj
<<
 /Length 44 
>>
stream
BT
/F1 100 Tf
10 400 Td
(Hello World!) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000060 00000 n 
0000000121 00000 n 
0000000278 00000 n 
trailer
<<
 /Root 1 0 R /Size 5 
>>
startxref
379
%%EOF`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pdf{
				contents: tt.fields.contents,
			}
			if got := p.fixXref(tt.args.b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fixXref() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestPdf_merge(t *testing.T) {
	type fields struct {
		contents []byte
	}
	type args struct {
		file Parasite
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				contents: []byte(`%PDF-1.3
1 0 obj
<<
 /Type /Catalog /Pages 2 0 R 
>>
endobj
2 0 obj
<<
 /Type /Pages /Kids [ 3 0 R ] /Count 1 
>>
endobj
3 0 obj
<<
 /Type /Page /Parent 2 0 R /Resources 
<<
 /Font 
<<
 /F1 
<<
 /Type /Font /Subtype /Type1 /BaseFont /Arial 
>>
>>
>>
 /Contents 4 0 R 
>>
endobj
4 0 obj
<<
 /Length 44 
>>
stream
BT
/F1 100 Tf
10 400 Td
(Hello World!) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000060 00000 n 
0000000121 00000 n 
0000000278 00000 n 
trailer
<<
 /Root 1 0 R /Size 5 
>>
startxref
379
%%EOF`),
			},
			args: args{
				file: func() Parasite {
					p, _ := NewPdf([]byte("%PDF-1.3qweeq"))
					return p
				}(),
			},
			want: []byte(`%PDF-1.3
1 0 obj
<</Length 2 0 R>>
stream
%PDF-1.3qweeq
endstream
endobj
2 0 obj
13
endobj
3 0 obj
<<
  /Payload 1 0 R
  /Pages 4 0 R
  /Type /Catalog
>>
endobj
4 0 obj
<<
  /Count 1
  /Kids[ 5 0 R ]
  /Type /Pages
>>
endobj`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pdf{
				contents: tt.fields.contents,
			}
			got, err := p.mergeTemplate(tt.args.file, 2)
			if (err != nil) != tt.wantErr {
				t.Errorf("mergeTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeTemplate() got = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestPdf_Infect(t *testing.T) {
	type fields struct {
		contents []byte
	}
	type args struct {
		file Parasite
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "t-05",
			fields: fields{
				contents: []byte(`%PDF-1.3
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [ 3 0 R ] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Arial >> >> >> /Contents 4 0 R >>
endobj
4 0 obj
<< /Length 44 >>
stream
BT
/F1 100 Tf
10 400 Td
(Hello World!) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000010 00000 n 
0000000060 00000 n 
0000000120 00000 n 
0000000269 00000 n 
trailer
<< /Root 1 0 R /Size 5 >>
startxref
364
%%EOF`),
			},
			args: args{
				file: func() Parasite {
					p, _ := NewPdf([]byte(`%PDF-3qweqew`))
					return p
				}(),
			},
			want: []byte(`%PDF-1.3
1 0 obj
<</Length 2 0 R>>
stream
%PDF-3qweqew
endstream
endobj
2 0 obj
12
endobj
3 0 obj
<<
  /Payload 1 0 R
  /Pages 4 0 R
  /Type /Catalog
>>
endobj
4 0 obj
<<
  /Count 1
  /Kids[ 5 0 R ]
  /Type /Pages
>>
endobj
5 0 obj
<<
 /Type /Page /Parent 4 0 R /Resources <<
 /Font 
<<
 /F1 
<<
 /Type /Font /Subtype /Type1 /BaseFont /Arial 
>>
>>
>>
 /Contents 6 0 R >>
endobj
6 0 obj
<<
 /Length 44 
>>
stream
BT
/F1 100 Tf
10 400 Td
(Hello World!) Tj
ET
endstream
endobj
xref
0 7
0000000000 65535 f 
0000000009 00000 n 
0000000072 00000 n 
0000000090 00000 n 
0000000160 00000 n 
0000000224 00000 n 
0000000379 00000 n 
trailer
<<
 /Root 3 0 R /Size 7
>>
startxref
480
%%EOF`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pdf{
				contents: tt.fields.contents,
			}
			got, err := p.Infect(tt.args.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("Infect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Infect() got = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
