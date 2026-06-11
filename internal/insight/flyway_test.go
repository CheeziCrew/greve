package insight

import (
	"path/filepath"
	"testing"
)

func TestLoadDBSchemaFixture(t *testing.T) {
	repo := filepath.Join("..", "..", "testdata", "repos", "api-service-alpha")
	schema, err := LoadDBSchema(repo, "api-service-alpha", true)
	if err != nil {
		t.Fatalf("LoadDBSchema: %v", err)
	}

	if len(schema.Tables) != 2 {
		t.Fatalf("tables = %d, want 2", len(schema.Tables))
	}

	tables := map[string]Table{}
	for _, table := range schema.Tables {
		tables[table.Name] = table
	}

	thing := tables["thing"]
	cols := map[string]string{}
	for _, col := range thing.Columns {
		cols[col.Name] = col.Type
	}
	if _, ok := cols["status"]; !ok {
		t.Error("thing.status missing (alter add column not applied)")
	}
	if _, ok := cols["created"]; ok {
		t.Error("thing.created present (alter drop column not applied)")
	}
	if _, ok := cols["constraint"]; ok {
		t.Error("'constraint' parsed as a column (add constraint leaked through)")
	}
	if len(thing.Indexes) != 1 || thing.Indexes[0] != "thing_name_index" {
		t.Errorf("thing.Indexes = %v, want [thing_name_index]", thing.Indexes)
	}

	detail := tables["thing_detail"]
	if len(detail.ForeignKeys) != 1 || detail.ForeignKeys[0] != "thing_id -> thing(id)" {
		t.Errorf("thing_detail FKs = %v, want [thing_id -> thing(id)]", detail.ForeignKeys)
	}
	for _, col := range detail.Columns {
		if col.Name == "detail" && col.Type != "varchar(2048)" {
			t.Errorf("detail column type = %q, want varchar(2048) (modify not applied)", col.Type)
		}
	}

	if len(schema.Migrations) != 2 {
		t.Errorf("migrations = %d, want 2", len(schema.Migrations))
	}
}
