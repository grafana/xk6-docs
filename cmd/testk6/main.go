// Command testk6 is a real k6 binary with the xk6-docs extension registered.
// It is built once by TestScripts and copied into each testscript workdir so
// that scripts can exercise the full k6 → extension router → docs path with
// exec ./k6 x docs ...
package main

import (
	"go.k6.io/k6/cmd"

	_ "github.com/grafana/xk6-docs"
)

func main() {
	cmd.Execute()
}
