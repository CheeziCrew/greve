package insight

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Table is the accumulated state of one database table after replaying all
// migrations in order.
type Table struct {
	Name        string   `json:"name"`
	Columns     []Column `json:"columns"`
	Indexes     []string `json:"indexes,omitempty"`
	ForeignKeys []string `json:"foreign_keys,omitempty"` // "col -> table(col)"
	CreatedIn   string   `json:"created_in"`             // migration version
}

// Column is one table column.
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Migration is one Flyway migration file.
type Migration struct {
	Version     string   `json:"version"`
	Description string   `json:"description"`
	File        string   `json:"file"`
	Tables      []string `json:"tables_touched,omitempty"`
}

// DBSchema is the result of replaying a service's migrations.
type DBSchema struct {
	Service    string      `json:"service"`
	Tables     []Table     `json:"tables"`
	Migrations []Migration `json:"migrations,omitempty"`
	// Parsing is regex-based and tuned for dept44-style DDL; exotic
	// statements are skipped, not misread.
	Note string `json:"note,omitempty"`
}

var migrationFileRe = regexp.MustCompile(`^V([0-9._]+)__(.+)\.sql$`)

// LoadDBSchema replays src/main/resources/db/migration in version order.
// withHistory includes the migration list in the result.
func LoadDBSchema(repoPath, serviceName string, withHistory bool) (*DBSchema, error) {
	dir := filepath.Join(repoPath, "src", "main", "resources", "db", "migration")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, e := range entries {
		m := migrationFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		migrations = append(migrations, Migration{
			Version:     strings.ReplaceAll(m[1], "_", "."),
			Description: strings.ReplaceAll(m[2], "_", " "),
			File:        e.Name(),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return versionLess(migrations[i].Version, migrations[j].Version)
	})

	tables := map[string]*Table{}
	var order []string
	for i := range migrations {
		migration := &migrations[i]
		data, err := os.ReadFile(filepath.Join(dir, migration.File))
		if err != nil {
			continue
		}
		touched := applyMigration(string(data), migration.Version, tables, &order)
		sort.Strings(touched)
		migration.Tables = touched
	}

	schema := &DBSchema{
		Service: serviceName,
		Note:    "regex-parsed from Flyway DDL; exotic statements are skipped",
	}
	for _, name := range order {
		if t, ok := tables[name]; ok {
			schema.Tables = append(schema.Tables, *t)
		}
	}
	if withHistory {
		schema.Migrations = migrations
	}
	return schema, nil
}

var (
	createTableRe  = regexp.MustCompile(`(?is)create\s+table\s+(?:if\s+not\s+exists\s+)?` + "`?" + `([a-z0-9_]+)` + "`?" + `\s*\((.*?)\)\s*(?:engine|;|$)`)
	dropTableRe    = regexp.MustCompile(`(?i)drop\s+table\s+(?:if\s+exists\s+)?` + "`?" + `([a-z0-9_]+)`)
	alterTableRe   = regexp.MustCompile(`(?is)alter\s+table\s+(?:if\s+exists\s+)?` + "`?" + `([a-z0-9_]+)` + "`?" + `\s+(.*?)(?:;|$)`)
	createIndexRe  = regexp.MustCompile(`(?i)create\s+(?:unique\s+)?index\s+(?:if\s+not\s+exists\s+)?` + "`?" + `([a-z0-9_]+)` + "`?" + `\s+on\s+` + "`?" + `([a-z0-9_]+)`)
	columnDefRe    = regexp.MustCompile("(?i)^`?([a-z0-9_]+)`?\\s+(.+?)(?:,)?$")
	addColumnRe    = regexp.MustCompile(`(?i)add\s+(?:column\s+)?` + "`?" + `([a-z0-9_]+)` + "`?" + `\s+([a-z0-9_()]+)`)
	dropColumnRe   = regexp.MustCompile(`(?i)drop\s+(?:column\s+)?` + "`?" + `([a-z0-9_]+)`)
	modifyColumnRe = regexp.MustCompile(`(?i)(?:modify|change)\s+(?:column\s+)?` + "`?" + `([a-z0-9_]+)` + "`?" + `\s+(?:` + "`?" + `[a-z0-9_]+` + "`?" + `\s+)?([a-z0-9_()]+)`)
	foreignKeyRe   = regexp.MustCompile(`(?i)foreign\s+key\s*\(` + "`?" + `([a-z0-9_]+)` + "`?" + `\)\s*references\s+` + "`?" + `([a-z0-9_]+)` + "`?" + `\s*\(` + "`?" + `([a-z0-9_]+)`)
)

// nonColumnPrefixes are create-table body lines that aren't column defs.
var nonColumnPrefixes = []string{"primary key", "constraint", "foreign key", "unique", "key ", "index ", "check"}

// ddlKeywords are "alter table add X" targets that aren't columns.
var ddlKeywords = map[string]bool{
	"constraint": true, "primary": true, "foreign": true,
	"unique": true, "index": true, "key": true, "fulltext": true, "check": true,
}

func applyMigration(sql, version string, tables map[string]*Table, order *[]string) []string {
	touched := map[string]bool{}

	for _, m := range createTableRe.FindAllStringSubmatch(sql, -1) {
		name, body := m[1], m[2]
		t := &Table{Name: name, CreatedIn: version}
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || isNonColumn(line) {
				for _, fk := range foreignKeyRe.FindAllStringSubmatch(line, -1) {
					t.ForeignKeys = append(t.ForeignKeys, fk[1]+" -> "+fk[2]+"("+fk[3]+")")
				}
				continue
			}
			if cm := columnDefRe.FindStringSubmatch(line); cm != nil {
				t.Columns = append(t.Columns, Column{Name: cm[1], Type: strings.TrimSuffix(strings.TrimSpace(cm[2]), ",")})
			}
		}
		tables[name] = t
		*order = append(*order, name)
		touched[name] = true
	}

	for _, m := range dropTableRe.FindAllStringSubmatch(sql, -1) {
		delete(tables, m[1])
		touched[m[1]] = true
	}

	for _, m := range alterTableRe.FindAllStringSubmatch(sql, -1) {
		name, body := m[1], m[2]
		t, ok := tables[name]
		if !ok {
			continue
		}
		touched[name] = true
		for _, add := range addColumnRe.FindAllStringSubmatch(body, -1) {
			if ddlKeywords[strings.ToLower(add[1])] {
				continue // "add constraint ...", "add primary key ...", etc.
			}
			t.Columns = append(t.Columns, Column{Name: add[1], Type: add[2]})
		}
		if !strings.Contains(strings.ToLower(body), "add") {
			for _, drop := range dropColumnRe.FindAllStringSubmatch(body, -1) {
				t.Columns = removeColumn(t.Columns, drop[1])
			}
		}
		for _, mod := range modifyColumnRe.FindAllStringSubmatch(body, -1) {
			for i := range t.Columns {
				if t.Columns[i].Name == mod[1] {
					t.Columns[i].Type = mod[2]
				}
			}
		}
		for _, fk := range foreignKeyRe.FindAllStringSubmatch(body, -1) {
			t.ForeignKeys = append(t.ForeignKeys, fk[1]+" -> "+fk[2]+"("+fk[3]+")")
		}
	}

	for _, m := range createIndexRe.FindAllStringSubmatch(sql, -1) {
		if t, ok := tables[m[2]]; ok {
			t.Indexes = append(t.Indexes, m[1])
			touched[m[2]] = true
		}
	}

	names := make([]string, 0, len(touched))
	for name := range touched {
		names = append(names, name)
	}
	return names
}

func isNonColumn(line string) bool {
	lower := strings.ToLower(line)
	for _, prefix := range nonColumnPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.HasPrefix(line, ")") || strings.HasPrefix(line, "(")
}

func removeColumn(columns []Column, name string) []Column {
	out := columns[:0]
	for _, col := range columns {
		if col.Name != name {
			out = append(out, col)
		}
	}
	return out
}

// versionLess compares dotted migration versions numerically.
func versionLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			return av < bv
		}
	}
	return false
}
