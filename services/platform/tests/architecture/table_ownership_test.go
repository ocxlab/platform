package architecture_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type ownershipException struct {
	Path   string `json:"path"`
	Query  string `json:"query"`
	Object string `json:"object"`
	Access string `json:"access"`
	Reason string `json:"reason"`
}
type ownershipManifest struct {
	Tables     map[string]string    `json:"tables"`
	Functions  map[string]string    `json:"functions"`
	Exceptions []ownershipException `json:"exceptions"`
}

// This is a conservative source guard, not a PostgreSQL parser or security boundary.
// Mask comments/literals before matching identifiers; keep quoted identifiers.
var sqlTokens = regexp.MustCompile(`(?s)--[^\n]*|/\*.*?\*/|'(?:''|[^'])*'|\$[A-Za-z_][A-Za-z_0-9]*\$.*?\$[A-Za-z_][A-Za-z_0-9]*\$|\$\$.*?\$\$|"(?:""|[^"])*"|[A-Za-z_][A-Za-z_0-9]*|[.(),;]`)
var queryHeader = regexp.MustCompile(`(?m)^-- name: ([A-Za-z_][A-Za-z_0-9]*) :[a-z]+\s*$`)

func tokensSQL(sql string) []string {
	var out []string
	for _, token := range sqlTokens.FindAllString(sql, -1) {
		if strings.HasPrefix(token, "--") || strings.HasPrefix(token, "/*") || strings.HasPrefix(token, "'") || strings.HasPrefix(token, "$") {
			continue
		}
		out = append(out, strings.ToLower(strings.ReplaceAll(strings.Trim(token, `"`), `""`, `"`)))
	}
	return out
}

// Application write targets must be schema-qualified. CTE names may be read
// unqualified, but write targets cannot be CTE aliases in the supported inputs.
func sqlAccess(sql string, m ownershipManifest) (map[string]string, error) {
	tokens := tokensSQL(sql)
	access := map[string]string{}
	for i := 0; i < len(tokens); i++ {
		if i+2 < len(tokens) && tokens[i+1] == "." {
			object := tokens[i] + "." + tokens[i+2]
			if slices.Contains([]string{"auth", "commerce", "commerce_internal", "payment", "payments", "runtime", "discovery", "shopping", "lodging"}, tokens[i]) && m.Tables[object] == "" && m.Functions[object] == "" {
				return nil, fmt.Errorf("unregistered application object %s", object)
			}
			if _, ok := m.Tables[object]; ok {
				if access[object] == "" {
					access[object] = "read"
				}
			}
			if _, ok := m.Functions[object]; ok && i+3 < len(tokens) && tokens[i+3] == "(" {
				access[object] = "call"
			}
		}
		write := tokens[i] == "into" || (tokens[i] == "update" && (i == 0 || (tokens[i-1] != "for" && tokens[i-1] != "do")))
		if slices.Contains([]string{"truncate", "merge", "alter", "create", "drop", "copy", "call"}, tokens[i]) {
			return nil, fmt.Errorf("unsupported application SQL operation %s; extend the guard before using it", tokens[i])
		}
		if tokens[i] == "delete" && i+1 < len(tokens) && tokens[i+1] == "from" {
			i++
			write = true
		}
		if write {
			if i+3 >= len(tokens) || tokens[i+2] != "." {
				return nil, fmt.Errorf("unqualified or unsupported SQL write target near %v", tokens[i:])
			}
			object := tokens[i+1] + "." + tokens[i+3]
			if _, ok := m.Tables[object]; !ok {
				return nil, fmt.Errorf("unregistered write target %s", object)
			}
			access[object] = "write"
		}
	}
	return access, nil
}

func ownerForQuery(path string) (string, error) {
	parts := strings.Split(path, "/")
	if len(parts) > 2 && parts[0] == "internal" && slices.Contains([]string{"core", "food", "shopping", "lodging", "payment", "discovery"}, parts[1]) {
		return parts[1], nil
	}
	if strings.HasPrefix(path, "cmd/api/transport/ucpinternal/persistence/queries/") {
		return "core", nil
	}
	return "", fmt.Errorf("unregistered query owner: %s", path)
}

func foreignAccess(path, query, sql string, m ownershipManifest) ([]ownershipException, error) {
	owner, err := ownerForQuery(path)
	if err != nil {
		return nil, err
	}
	access, err := sqlAccess(sql, m)
	if err != nil {
		return nil, err
	}
	var foreign []ownershipException
	for object, mode := range access {
		other := m.Tables[object]
		if other == "" {
			other = m.Functions[object]
		}
		if other != owner {
			foreign = append(foreign, ownershipException{Path: path, Query: query, Object: object, Access: mode})
		}
	}
	return foreign, nil
}

func matchesException(actual, allowed ownershipException) bool {
	return actual.Path == allowed.Path && actual.Query == allowed.Query && actual.Object == allowed.Object && actual.Access == allowed.Access && strings.TrimSpace(allowed.Reason) != ""
}

func TestQueryTableOwnership(t *testing.T) {
	data, err := os.ReadFile("ownership.json")
	if err != nil {
		t.Fatal(err)
	}
	var m ownershipManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, objects := range []map[string]string{m.Tables, m.Functions} {
		for object, owner := range objects {
			if !slices.Contains([]string{"core", "food", "shopping", "lodging", "payment", "discovery"}, owner) {
				t.Fatalf("invalid logical owner %q for %s", owner, object)
			}
		}
	}
	seenExceptions := map[string]bool{}
	for _, exception := range m.Exceptions {
		key := exception.Path + "/" + exception.Query + "/" + exception.Object + "/" + exception.Access
		if seenExceptions[key] {
			t.Fatalf("duplicate exception %s", key)
		}
		seenExceptions[key] = true
	}
	used := make([]bool, len(m.Exceptions))
	root := filepath.Join("..", "..")
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && slices.Contains([]string{"sqlgen", ".git"}, entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".sql") || !strings.Contains(filepath.ToSlash(path), "/queries/") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sql := string(data)
		headers := queryHeader.FindAllStringSubmatchIndex(sql, -1)
		if len(headers) == 0 {
			t.Errorf("query input without named statements: %s", relative)
		}
		for i, h := range headers {
			end := len(sql)
			if i+1 < len(headers) {
				end = headers[i+1][0]
			}
			query := sql[h[2]:h[3]]
			foreign, err := foreignAccess(relative, query, sql[h[1]:end], m)
			if err != nil {
				t.Errorf("%s/%s: %v", relative, query, err)
				continue
			}
			for _, actual := range foreign {
				found := false
				for j, allowed := range m.Exceptions {
					if matchesException(actual, allowed) {
						used[j] = true
						found = true
					}
				}
				if !found {
					t.Errorf("unapproved cross-owner access: %+v", actual)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, seen := range used {
		if !seen {
			t.Errorf("unused ownership exception: %+v", m.Exceptions[i])
		}
	}
	// New migration objects need a declared logical owner, even before a query uses them.
	create := regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?(TABLE|FUNCTION)\s+([a-z_]+\.[a-z_]+)`)
	files, err := filepath.Glob(filepath.Join(root, "internal/infra/database/migrations/*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range create.FindAllStringSubmatch(string(data), -1) {
			objects := m.Tables
			if strings.EqualFold(match[1], "FUNCTION") {
				objects = m.Functions
			}
			if objects[strings.ToLower(match[2])] == "" {
				t.Errorf("migration object has no logical owner: %s", match[2])
			}
		}
	}
}

func TestOwnershipGuardNegativeFixtures(t *testing.T) {
	m := ownershipManifest{Tables: map[string]string{"commerce.stores": "core", "commerce.food_orders": "food"}, Functions: map[string]string{"runtime.finish_effect": "core"}}
	for _, fixture := range []struct{ sql, mode string }{
		{`UPDATE commerce.stores SET name='hello'`, "write"},
		{`WITH x AS (DELETE FROM "commerce"."stores" RETURNING *) SELECT * FROM x`, "write"},
		{`INSERT /* comment */ INTO commerce.stores VALUES (1)`, "write"},
		{`SELECT runtime.finish_effect(1)`, "call"},
		{`SELECT * FROM commerce.stores`, "read"},
	} {
		foreign, err := foreignAccess("internal/food/persistence/queries/test.sql", "Attack", fixture.sql, m)
		if err != nil || len(foreign) != 1 || foreign[0].Access != fixture.mode {
			t.Fatalf("missed foreign access %q: %v, %v", fixture.sql, foreign, err)
		}
	}
	for _, sql := range []string{`UPDATE stores SET name='x'`, `INSERT INTO unknown.items VALUES (1)`, `TRUNCATE commerce.stores`, `SELECT runtime.unregistered(1)`} {
		if _, err := sqlAccess(sql, m); err == nil {
			t.Errorf("accepted unsupported write: %s", sql)
		}
	}
	access, err := sqlAccess(`SELECT 'UPDATE commerce.stores' FROM commerce.food_orders /* DELETE FROM commerce.stores */ FOR UPDATE`, m)
	if err != nil || len(access) != 1 || access["commerce.food_orders"] != "read" {
		t.Fatalf("literal/comment/row-lock false positive: %v, %v", access, err)
	}
	if _, err := ownerForQuery("internal/newdomain/persistence/queries/test.sql"); err == nil {
		t.Fatal("accepted unregistered query owner")
	}
}

func TestOwnershipExceptionsDoNotGrantBlanketAccess(t *testing.T) {
	allowed := ownershipException{Path: "internal/food/persistence/queries/test.sql", Query: "Allowed", Object: "commerce.stores", Access: "read", Reason: "store existence"}
	if !matchesException(allowed, allowed) {
		t.Fatal("exact approved access rejected")
	}
	for _, field := range []string{"path", "query", "object", "access", "reason"} {
		changed := allowed
		switch field {
		case "path":
			changed.Path = "internal/food/persistence/queries/other.sql"
		case "query":
			changed.Query = "NewQuery"
		case "object":
			changed.Object = "auth.grants"
		case "access":
			changed.Access = "write"
		case "reason":
			changed.Reason = " "
		}
		if matchesException(allowed, changed) {
			t.Errorf("exception leaked across %s", field)
		}
	}
}
