package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CheeziCrew/greve/internal/scan"
)

func schemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema <service> <METHOD> <path|operationId>",
		Short: "Resolved request/response schema for one endpoint ($refs inlined)",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := loadCatalog()
			if err != nil {
				return err
			}
			s := c.Lookup(args[0])
			if s == nil {
				return fmt.Errorf("no service matching %q", args[0])
			}
			if s.SpecPath == "" {
				return fmt.Errorf("%s has no OpenAPI spec on disk", s.Name)
			}

			method := strings.ToUpper(args[1])
			path, operationID := args[2], ""
			if !strings.HasPrefix(path, "/") {
				path, operationID = "", args[2]
			}

			op, err := scan.LoadOperationSchema(
				filepath.Join(s.Path, filepath.FromSlash(s.SpecPath)), method, path, operationID)
			if err != nil {
				return err
			}
			op.Service = s.Name
			return printJSON(op)
		},
	}
	return cmd
}
