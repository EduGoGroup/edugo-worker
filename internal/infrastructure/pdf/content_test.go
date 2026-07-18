package pdf

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTextFromContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "Tj simple",
			content: `BT /F0 12 Tf (Hola mundo) Tj ET`,
			want:    "Hola mundo",
		},
		{
			name:    "acentos WinAnsi latin-1",
			content: "BT (fotos\xedntesis y energ\xeda) Tj ET",
			want:    "fotosíntesis y energía",
		},
		{
			name:    "comillas tipograficas WinAnsi 0x93 0x94",
			content: "BT (\x93cita\x94) Tj ET",
			want:    "“cita”",
		},
		{
			name:    "escapes de parentesis y backslash",
			content: `BT (a\(b\)c\\d) Tj ET`,
			want:    `a(b)c\d`,
		},
		{
			name:    "escape octal",
			content: `BT (\101\102\103) Tj ET`, // 'A' 'B' 'C' en octal
			want:    "ABC",
		},
		{
			name:    "hex string",
			content: `BT <48656C6C6F> Tj ET`, // "Hello"
			want:    "Hello",
		},
		{
			name:    "TJ array con ajuste de espacio",
			content: `BT [(Hola)-400(mundo)] TJ ET`,
			want:    "Hola mundo",
		},
		{
			name:    "TJ array con kerning pequeno sin espacio",
			content: `BT [(Ho)-40(la)] TJ ET`,
			want:    "Hola",
		},
		{
			name:    "operador comilla simple salto de linea",
			content: `BT (linea1) Tj (linea2) ' ET`,
			want:    "linea1\nlinea2",
		},
		{
			name:    "T-star inserta salto",
			content: `BT (a) Tj T* (b) Tj ET`,
			want:    "a\nb",
		},
		{
			name:    "Td vertical inserta salto",
			content: `BT (a) Tj 0 -14 Td (b) Tj ET`,
			want:    "a\nb",
		},
		{
			name:    "comentario ignorado",
			content: "BT (texto) Tj ET\n% esto es comentario\n",
			want:    "texto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextFromContent([]byte(tt.content))
			assert.Equal(t, tt.want, strings.TrimSpace(got))
		})
	}
}

func TestDecodeWinAnsi(t *testing.T) {
	// ASCII intacto
	assert.Equal(t, "abc123", decodeWinAnsi([]byte("abc123")))
	// Latin-1 alto (0xF1 = ñ, 0xFC = ü, 0xBF = ¿)
	assert.Equal(t, "ñü¿", decodeWinAnsi([]byte{0xF1, 0xFC, 0xBF}))
	// Especiales Windows-1252 (0x91/0x92 comillas simples, 0x97 em-dash)
	assert.Equal(t, "‘’—", decodeWinAnsi([]byte{0x91, 0x92, 0x97}))
}
