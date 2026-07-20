package materialpipeline

import "testing"

// Enunciados con referencia deíctica al contexto del prompt: se detectan aunque varíen
// mayúsculas, acentos o espacios (casos reales del draft CONASET ca895dd4 incluidos).
func TestDetectDeicticReference_Positivos(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"caso real CONASET ideas proporcionadas", "¿Cuál es el costo total asociado a los accidentes de tránsito según las ideas proporcionadas?"},
		{"caso real CONASET segun las ideas", "¿Qué aspecto general del sistema requiere atención constante según las ideas?"},
		{"mayusculas y acentos", "SEGÚN EL TEXTO, ¿qué distancia debe mantener?"},
		{"de acuerdo con el texto", "De acuerdo con el texto, ¿cuándo se usa la luz alta?"},
		{"texto anterior", "Como indica el texto anterior, ¿qué es la conducción defensiva?"},
		{"en el material", "¿Qué señales se describen en el material?"},
		{"material proporcionado", "Según el material proporcionado, ¿cuál es la velocidad máxima urbana?"},
		{"lo visto", "Según lo visto, ¿qué elementos componen la vía?"},
		{"visto anteriormente", "Relacione lo visto anteriormente con la señalización vertical."},
		{"mencionado anteriormente", "El factor humano, mencionado anteriormente, ¿qué porcentaje de accidentes explica?"},
		{"mencionadas anteriormente", "De las causas mencionadas anteriormente, ¿cuál es la principal?"},
		{"fragmento", "Según el fragmento, ¿qué obligación tiene el peatón?"},
		{"espacios multiples", "según   las\n\tideas dadas, ¿qué es la calzada?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectDeicticReference(tc.text); got == "" {
				t.Fatalf("DetectDeicticReference(%q) = \"\", quiero una frase detectada", tc.text)
			}
		})
	}
}

// Enunciados legítimos y autocontenidos que NO deben dispararse: «según» anclado a
// fuentes reales (ley, reglamento, señal) es normal en un manual de conducir.
func TestDetectDeicticReference_FalsosPositivos(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"segun la ley", "Según la ley de tránsito, ¿cuál es el límite de alcohol permitido?"},
		{"segun el reglamento", "Según el reglamento, ¿quién tiene preferencia en un cruce sin señalizar?"},
		{"segun la normativa", "Según la normativa vigente, ¿a qué edad se puede obtener licencia clase B?"},
		{"texto de una señal", "¿Qué significa el texto de la señal PARE?"},
		{"materiales fisicos", "¿De qué material están hechas las señales reflectantes?"},
		{"ideas como concepto del contenido", "¿Qué ideas promueve la conducción eficiente?"},
		{"anterior sin deixis de discurso", "Si el semáforo anterior estaba en rojo, ¿qué debe esperar del siguiente?"},
		{"parrafo de la ley", "El artículo 110, en el párrafo segundo, ¿qué establece sobre la velocidad?"},
		{"enunciado autocontenido normal", "¿Cuál es la distancia mínima de seguimiento en carretera con lluvia?"},
		{"vacio", ""},
		{"solo espacios", "   \n\t "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectDeicticReference(tc.text); got != "" {
				t.Fatalf("DetectDeicticReference(%q) = %q, quiero \"\" (falso positivo)", tc.text, got)
			}
		})
	}
}
