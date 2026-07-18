package pdf

import "strings"

// content.go — parser mínimo de content-streams PDF para extraer texto real.
//
// pdfcpu entrega el content-stream ya descomprimido (operadores + operandos),
// pero NO extrae texto: `ExtractPageContent` devuelve cosas como
//
//	q BT /F0 10 Tf ET ... 0 3 Td (El ciclo del agua) Tj ET Q
//
// El texto visible vive en los operandos string de los operadores de texto
// (`Tj`, `TJ`, `'`, `"`). Este parser recorre el stream, decodifica esos
// strings (literales `(...)` y hex `<...>`) desde WinAnsi y los emite,
// insertando saltos de línea en los operadores de posicionamiento de texto.
//
// Cobertura y límites (documentados a propósito):
//   - Operadores de texto: Tj, TJ, ' (comilla simple), " (comilla doble).
//   - Saltos de línea heurísticos: T*, ', ", ET, y Td/TD con desplazamiento
//     vertical (ty != 0). Td/TD con solo desplazamiento horizontal grande
//     inserta un espacio.
//   - Ajustes de TJ: un número suficientemente negativo (gap hacia la derecha)
//     inserta un espacio entre glifos (separación de palabras).
//   - Codificación: WinAnsi (Windows-1252). Cubre español y latín-1 completos.
//     NO resuelve fuentes con Encoding personalizado ni CID/Identity-H (Type0):
//     esos requieren mapear el CMap ToUnicode, fuera de alcance de este parser.
//   - Se saltan comentarios `%` y bloques de imagen inline `BI ... ID ... EI`.
const (
	// tjSpaceThreshold: en un arreglo TJ, un ajuste <= -tjSpaceThreshold
	// (milésimas de unidad de texto) se interpreta como separación de palabra.
	tjSpaceThreshold = 100.0
	// tdSpaceThreshold: un Td/TD puramente horizontal con tx >= este valor se
	// interpreta como un espacio (sangría/tabulación); por debajo se ignora.
	tdSpaceThreshold = 1.0
)

// operand es un objeto del stack de operandos: texto decodificado, número o arreglo.
type operand struct {
	isText  bool
	isNum   bool
	isArray bool
	text    string
	num     float64
	arr     []operand
}

// extractTextFromContent parsea un content-stream PDF ya descomprimido y
// devuelve el texto visible (sin limpiar). Ver cobertura arriba.
func extractTextFromContent(content []byte) string {
	var out strings.Builder
	stack := make([]operand, 0, 8)
	i, n := 0, len(content)

	for i < n {
		c := content[i]
		switch {
		case isPDFWhitespace(c):
			i++
		case c == '%':
			// comentario hasta fin de línea
			for i < n && content[i] != '\n' && content[i] != '\r' {
				i++
			}
		case c == '(':
			s, ni := readLiteralString(content, i)
			stack = append(stack, operand{isText: true, text: s})
			i = ni
		case c == '<':
			if i+1 < n && content[i+1] == '<' {
				// diccionario: no esperado en content-stream; saltar operandos
				i += 2
				stack = stack[:0]
			} else {
				s, ni := readHexString(content, i)
				stack = append(stack, operand{isText: true, text: s})
				i = ni
			}
		case c == '[':
			arr, ni := readArray(content, i)
			stack = append(stack, operand{isArray: true, arr: arr})
			i = ni
		case c == '/':
			_, ni := readName(content, i)
			stack = append(stack, operand{}) // placeholder de nombre
			i = ni
		case c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9'):
			f, ni := readNumber(content, i)
			stack = append(stack, operand{isNum: true, num: f})
			i = ni
		default:
			op, ni := readOperator(content, i)
			i = ni
			if op == "BI" {
				// imagen inline: saltar hasta el token EI
				i = skipInlineImage(content, i)
				stack = stack[:0]
				continue
			}
			handleTextOp(op, stack, &out)
			stack = stack[:0]
		}
	}

	return out.String()
}

// handleTextOp reacciona a un operador consumiendo el stack de operandos.
func handleTextOp(op string, stack []operand, out *strings.Builder) {
	switch op {
	case "Tj":
		if s, ok := lastText(stack); ok {
			out.WriteString(s)
		}
	case "'":
		// mover a la línea siguiente y mostrar el string
		out.WriteByte('\n')
		if s, ok := lastText(stack); ok {
			out.WriteString(s)
		}
	case "\"":
		// aw ac string " — mover a la línea siguiente y mostrar
		out.WriteByte('\n')
		if s, ok := lastText(stack); ok {
			out.WriteString(s)
		}
	case "TJ":
		if len(stack) > 0 && stack[len(stack)-1].isArray {
			writeTJArray(stack[len(stack)-1].arr, out)
		}
	case "T*":
		out.WriteByte('\n')
	case "Td", "TD":
		// operandos: tx ty Td
		if len(stack) >= 2 && stack[len(stack)-1].isNum && stack[len(stack)-2].isNum {
			ty := stack[len(stack)-1].num
			tx := stack[len(stack)-2].num
			switch {
			case ty != 0:
				out.WriteByte('\n')
			case tx >= tdSpaceThreshold:
				out.WriteByte(' ')
			}
		}
	case "ET":
		out.WriteByte('\n')
	}
}

// writeTJArray emite el texto de un arreglo TJ, insertando espacios donde el
// ajuste de posición indica una separación de palabra.
func writeTJArray(arr []operand, out *strings.Builder) {
	for _, o := range arr {
		switch {
		case o.isText:
			out.WriteString(o.text)
		case o.isNum && o.num <= -tjSpaceThreshold:
			out.WriteByte(' ')
		}
	}
}

func lastText(stack []operand) (string, bool) {
	if len(stack) > 0 && stack[len(stack)-1].isText {
		return stack[len(stack)-1].text, true
	}
	return "", false
}

// readLiteralString lee un string literal `(...)` con paréntesis balanceados y
// escapes PDF, y lo decodifica desde WinAnsi. b[i] debe ser '('.
func readLiteralString(b []byte, i int) (string, int) {
	i++ // saltar '('
	depth := 1
	raw := make([]byte, 0, 32)
	n := len(b)
	for i < n {
		c := b[i]
		if c == '\\' {
			i++
			if i >= n {
				break
			}
			e := b[i]
			switch e {
			case 'n':
				raw = append(raw, '\n')
				i++
			case 'r':
				raw = append(raw, '\r')
				i++
			case 't':
				raw = append(raw, '\t')
				i++
			case 'b':
				raw = append(raw, '\b')
				i++
			case 'f':
				raw = append(raw, '\f')
				i++
			case '(':
				raw = append(raw, '(')
				i++
			case ')':
				raw = append(raw, ')')
				i++
			case '\\':
				raw = append(raw, '\\')
				i++
			case '\r':
				// continuación de línea: '\' + EOL => nada
				i++
				if i < n && b[i] == '\n' {
					i++
				}
			case '\n':
				i++
			default:
				if e >= '0' && e <= '7' {
					oct, cnt := 0, 0
					for cnt < 3 && i < n && b[i] >= '0' && b[i] <= '7' {
						oct = oct*8 + int(b[i]-'0')
						i++
						cnt++
					}
					raw = append(raw, byte(oct))
				} else {
					// escape desconocido: el carácter se toma literal
					raw = append(raw, e)
					i++
				}
			}
			continue
		}
		switch c {
		case '(':
			depth++
			raw = append(raw, c)
			i++
		case ')':
			depth--
			if depth == 0 {
				i++
				return decodeWinAnsi(raw), i
			}
			raw = append(raw, c)
			i++
		default:
			raw = append(raw, c)
			i++
		}
	}
	return decodeWinAnsi(raw), i
}

// readHexString lee un string hex `<...>` y lo decodifica desde WinAnsi.
// b[i] debe ser '<' (y no '<<').
func readHexString(b []byte, i int) (string, int) {
	i++ // saltar '<'
	n := len(b)
	raw := make([]byte, 0, 16)
	hi, haveHi := 0, false
	for i < n {
		c := b[i]
		if c == '>' {
			i++
			break
		}
		if isPDFWhitespace(c) {
			i++
			continue
		}
		v := hexVal(c)
		if v < 0 {
			i++
			continue
		}
		if !haveHi {
			hi, haveHi = v, true
		} else {
			raw = append(raw, byte(hi<<4|v))
			haveHi = false
		}
		i++
	}
	if haveHi {
		// dígito impar final: se completa con 0
		raw = append(raw, byte(hi<<4))
	}
	return decodeWinAnsi(raw), i
}

// readArray lee un arreglo `[...]` con strings y números (para TJ). b[i]=='['.
func readArray(b []byte, i int) ([]operand, int) {
	i++ // saltar '['
	n := len(b)
	arr := make([]operand, 0, 8)
	for i < n {
		c := b[i]
		switch {
		case isPDFWhitespace(c):
			i++
		case c == ']':
			i++
			return arr, i
		case c == '(':
			s, ni := readLiteralString(b, i)
			arr = append(arr, operand{isText: true, text: s})
			i = ni
		case c == '<':
			s, ni := readHexString(b, i)
			arr = append(arr, operand{isText: true, text: s})
			i = ni
		case c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9'):
			f, ni := readNumber(b, i)
			arr = append(arr, operand{isNum: true, num: f})
			i = ni
		default:
			i++ // token inesperado dentro del arreglo: saltar
		}
	}
	return arr, i
}

// readName lee un nombre PDF `/Name` y devuelve su texto (sin el '/').
func readName(b []byte, i int) (string, int) {
	i++ // saltar '/'
	start := i
	n := len(b)
	for i < n && !isPDFWhitespace(b[i]) && !isPDFDelimiter(b[i]) {
		i++
	}
	return string(b[start:i]), i
}

// readNumber lee un número (entero o real) y lo devuelve como float64.
func readNumber(b []byte, i int) (float64, int) {
	start := i
	n := len(b)
	if i < n && (b[i] == '-' || b[i] == '+') {
		i++
	}
	for i < n && ((b[i] >= '0' && b[i] <= '9') || b[i] == '.') {
		i++
	}
	return parseFloat(b[start:i]), i
}

// readOperator lee un token de operador (secuencia de bytes que no son
// espacios ni delimitadores).
func readOperator(b []byte, i int) (string, int) {
	start := i
	n := len(b)
	for i < n && !isPDFWhitespace(b[i]) && !isPDFDelimiter(b[i]) {
		i++
	}
	if i == start {
		// byte delimitador suelto (p.ej. no consumido): avanzar uno
		i++
	}
	return string(b[start:i]), i
}

// skipInlineImage salta desde después de `BI` hasta el token `EI`.
func skipInlineImage(b []byte, i int) int {
	n := len(b)
	for i < n-1 {
		if b[i] == 'E' && b[i+1] == 'I' &&
			(i+2 >= n || isPDFWhitespace(b[i+2])) &&
			(i == 0 || isPDFWhitespace(b[i-1])) {
			return i + 2
		}
		i++
	}
	return n
}

// parseFloat convierte una porción de bytes numéricos a float64 sin usar
// strconv sobre strings intermedios costosos; tolera formatos PDF simples.
func parseFloat(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	neg := false
	i := 0
	if b[0] == '-' {
		neg = true
		i++
	} else if b[0] == '+' {
		i++
	}
	var intPart float64
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		intPart = intPart*10 + float64(b[i]-'0')
		i++
	}
	var frac, scale float64 = 0, 1
	if i < len(b) && b[i] == '.' {
		i++
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			frac = frac*10 + float64(b[i]-'0')
			scale *= 10
			i++
		}
	}
	v := intPart + frac/scale
	if neg {
		return -v
	}
	return v
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

func isPDFWhitespace(c byte) bool {
	switch c {
	case 0x00, 0x09, 0x0A, 0x0C, 0x0D, 0x20:
		return true
	default:
		return false
	}
}

func isPDFDelimiter(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}

// decodeWinAnsi mapea bytes WinAnsi (Windows-1252) a una string UTF-8.
func decodeWinAnsi(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(winAnsiTable[c])
	}
	return sb.String()
}

// winAnsiTable: WinAnsi == Windows-1252. 0x00-0x7F es ASCII; 0xA0-0xFF coincide
// con Latin-1 (rune == byte); 0x80-0x9F tiene la tabla especial de Windows-1252.
var winAnsiTable = buildWinAnsiTable()

func buildWinAnsiTable() [256]rune {
	var t [256]rune
	for i := 0; i < 256; i++ {
		t[i] = rune(i)
	}
	specials := map[byte]rune{
		0x80: '€', // €
		0x82: '‚', // ‚
		0x83: 'ƒ', // ƒ
		0x84: '„', // „
		0x85: '…', // …
		0x86: '†', // †
		0x87: '‡', // ‡
		0x88: 'ˆ', // ˆ
		0x89: '‰', // ‰
		0x8A: 'Š', // Š
		0x8B: '‹', // ‹
		0x8C: 'Œ', // Œ
		0x8E: 'Ž', // Ž
		0x91: '‘', // '
		0x92: '’', // '
		0x93: '“', // "
		0x94: '”', // "
		0x95: '•', // •
		0x96: '–', // –
		0x97: '—', // —
		0x98: '˜', // ˜
		0x99: '™', // ™
		0x9A: 'š', // š
		0x9B: '›', // ›
		0x9C: 'œ', // œ
		0x9E: 'ž', // ž
		0x9F: 'Ÿ', // Ÿ
	}
	for b, r := range specials {
		t[b] = r
	}
	return t
}
