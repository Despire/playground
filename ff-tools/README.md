# ff-tools

Fileformat tools this is not a generic tool to generate polyglot files. This implements the techniques described
in /paper/paper.pdf. Merging PDF with other and creating a PDF To Wasm encryption polyglot doesn't work automatically
and need to be adjusted manually due writting a correct PDF parser was out of scope for this.

to create polyglots files you can run the program:

```go
go run ./cmd/main merge ./test-input/rbg.png ./test-input/z.zip
```

there are are other commands such as pngToPdf or pdfToWasm that tries to create polyglots with AES-CBC encryption.
The supported formats are MP3, NES, PDF, ZIP, PNG, JPEG, GIF.
