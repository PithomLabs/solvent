package main

import "html/template"

var templateFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
}
