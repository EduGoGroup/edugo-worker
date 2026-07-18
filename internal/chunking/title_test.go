package chunking

import "testing"

func TestIsTitleLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"vacía", "", false},
		{"todo mayúsculas", "INTRODUCCIÓN A LA BIOLOGÍA", true},
		{"mayúsculas con eñe", "EL AÑO DE LA COSECHA", true},
		{"numerado simple", "1. La célula", true},
		{"numerado con subsección", "1.2 Membrana plasmática", true},
		{"romano", "II. La revolución industrial", true},
		{"keyword capítulo", "Capítulo 3: El sistema solar", true},
		{"keyword sección", "Sección de repaso", true},
		{"keyword tema", "Tema 4 — La fotosíntesis", true},
		{"keyword unidad", "Unidad 2", true},
		{"keyword inglés chapter", "Chapter One", true},
		{"línea corta sin punto", "Los verbos irregulares", true},
		{"frase normal con punto", "La célula es la unidad básica de la vida.", false},
		{"párrafo largo", "En este apartado analizaremos con cierto detalle cómo las plantas transforman la energía luminosa en energía química mediante un proceso conocido", false},
		{"viñeta con guion", "- primer punto de la lista", false},
		{"viñeta con asterisco", "* otro ítem de la lista", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isTitleLine(c.line); got != c.want {
				t.Errorf("isTitleLine(%q) = %v, quería %v", c.line, got, c.want)
			}
		})
	}
}

func TestIsAllUpper(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"HOLA MUNDO", true},
		{"ÁRBOL AÑEJO", true},
		{"Hola", false},
		{"HOLA mundo", false},
		{"12345", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isAllUpper(c.s); got != c.want {
			t.Errorf("isAllUpper(%q) = %v, quería %v", c.s, got, c.want)
		}
	}
}
