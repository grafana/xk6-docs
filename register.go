// Package module contains the xk6-docs extension.
package module

import "go.k6.io/k6/v2/subcommand"

func init() {
	subcommand.RegisterExtension("docs", newCmd)
}
