//go:build ignore

// Command gen_pdf regenera los PDF de testdata del harness modo material a partir de
// los .txt hermanos de esta carpeta. PROCEDENCIA de los PDF: se generan aquí, con
// pdfcpu (api.Create, la misma librería vendorizada que usa el extractor del worker),
// a partir de contenido educativo real en español escrito a mano en los .txt. No
// provienen de terceros ni tienen derechos de autor.
//
// Uso (desde edugo-worker/):
//
//	go run ./cmd/llm-harness/testdata/material/gen_pdf.go
//
// Cada línea del .txt se envuelve a ~95 caracteres y se coloca como un textbox; el
// resultado es un PDF de una página por documento. El objetivo es ejercitar el camino
// pdf.Extractor del harness; el CONTENIDO de estudio es idéntico al del .txt hermano.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

const wrapWidth = 95

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	// Permite correr el generador desde la raíz del módulo o desde la propia carpeta.
	base := dir
	if !strings.HasSuffix(dir, filepath.Join("testdata", "material")) {
		base = filepath.Join(dir, "cmd", "llm-harness", "testdata", "material")
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		txtPath := filepath.Join(base, e.Name())
		pdfPath := strings.TrimSuffix(txtPath, ".txt") + ".pdf"
		if err := genOne(txtPath, pdfPath); err != nil {
			fatal(fmt.Errorf("%s: %w", e.Name(), err))
		}
		fmt.Printf("generado %s\n", filepath.Base(pdfPath))
	}
}

func genOne(txtPath, pdfPath string) error {
	lines, err := wrappedLines(txtPath)
	if err != nil {
		return err
	}

	var boxes []string
	y := 810
	for _, ln := range lines {
		boxes = append(boxes, fmt.Sprintf(`{"value":%s,"pos":[1,%d],"font":{"name":"Helvetica","size":10}}`, jsonString(ln), y))
		y -= 13
	}
	spec := `{"paper":"A4P","origin":"LowerLeft","margin":{"width":8},"pages":{"1":{"content":{"text":[` +
		strings.Join(boxes, ",") + `]}}}}`

	out, err := os.Create(pdfPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	return api.Create(nil, strings.NewReader(spec), out, nil)
}

// wrappedLines lee el .txt y devuelve sus líneas envueltas a wrapWidth caracteres,
// preservando líneas en blanco como separadores de párrafo.
func wrappedLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " ")
		if strings.TrimSpace(line) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, wrap(line, wrapWidth)...)
	}
	return out, sc.Err()
}

func wrap(s string, width int) []string {
	words := strings.Fields(s)
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() > 0 && cur.Len()+1+len(w) > width {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// jsonString serializa s como literal JSON (comillas y escapes) para incrustarlo en el
// spec de pdfcpu sin depender de encoding/json.Marshal por un solo string.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
