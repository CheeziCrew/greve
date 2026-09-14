package review

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// scanCtx carries cross-file information a rule may need (e.g. the set of test
// class names for companion-test checks).
type scanCtx struct {
	TestClasses map[string]bool // basenames without ".java", e.g. "FooResourceFailureTest"
	// MigrationColumns is the Flyway-replayed schema: table -> column -> type ("varchar(255)").
	// Lets entity rules cross-check @Column against the actual DB column.
	MigrationColumns map[string]map[string]string
}

// hit is a rule match before it is stamped with rule metadata by the engine.
type hit struct {
	Line    int
	Message string
	Fix     string
}

// rule is one deterministic check. CorpusID cross-references the standards
// corpus entry (Capability B) that documents the same convention.
type rule struct {
	ID       string
	CorpusID string
	Title    string
	Default  Severity
	// Test marks a rule that lints test sources (src/test, src/integration-test)
	// instead of src/main — for conventions that only live in test code.
	Test  bool
	check func(f *javaFile, ctx *scanCtx) []hit
}

var (
	reTypeDecl   = regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+|final\s+|abstract\s+|sealed\s+|non-sealed\s+|static\s+)*(?:class|interface|enum|record)\s+\w+`)
	reZalando    = regexp.MustCompile(`^\s*import\s+org\.zalando\.problem`)
	reWildcard   = regexp.MustCompile(`^\s*import\s+(?:static\s+)?[\w.]+\.\*\s*;`)
	reLombok     = regexp.MustCompile(`^\s*import\s+lombok\.`)
	reScheduled  = regexp.MustCompile(`@Scheduled\b`)
	reEnumDecl   = regexp.MustCompile(`\benum\s+[A-Z]\w*\b`)
	reFeignAnno  = regexp.MustCompile(`@FeignClient\b`)
	reRepoIface  = regexp.MustCompile(`interface\s+\w+\s+extends\s+[^{]*\b(?:JpaRepository|CrudRepository|PagingAndSortingRepository)\b`)
	reAutowired  = regexp.MustCompile(`@Autowired\b`)
	rePublicRes  = regexp.MustCompile(`\bpublic\s+(?:final\s+)?class\s+\w*Resource\b`)
	rePathVarArg = regexp.MustCompile(`@PathVariable\s*\(\s*(?:"|value\s*=|name\s*=)`)
	reQualStatus = regexp.MustCompile(`\bHttpStatus\.[A-Z][A-Z0-9_]+\b`)
	reImportLine = regexp.MustCompile(`^\s*import\s`)
	reGenericWld = regexp.MustCompile(`\?\s*extends\b|\?\s*super\b|<\s*\?|\?\s*>`)
	reFqnAssertj  = regexp.MustCompile(`\borg\.assertj\.core\.(?:api\.Assertions|groups\.Tuple)\.`)
	reFqnHamcrest = regexp.MustCompile(`\borg\.hamcrest\.MatcherAssert\.assertThat\b`)
	reTableName   = regexp.MustCompile(`@Table\s*\(\s*name\s*=\s*"([a-z0-9_]+)"`)
	reColName     = regexp.MustCompile(`name\s*=\s*"([a-z0-9_]+)"`)
	reColLength   = regexp.MustCompile(`\blength\s*=\s*([0-9]+)`)
	reVarcharLen  = regexp.MustCompile(`(?i)varchar\s*\(\s*([0-9]+)\s*\)`)

	reController          = regexp.MustCompile(`\bclass\s+(\w*Controller)\b`)
	reRestCtrlAnno        = regexp.MustCompile(`@(?:Rest)?Controller\b`)
	reCollectToList       = regexp.MustCompile(`\.collect\(\s*(?:Collectors\.)?toList\(\s*\)\s*\)`)
	reJakartaTx           = regexp.MustCompile(`^\s*import\s+jakarta\.transaction\.Transactional\s*;`)
	reEntityAnno          = regexp.MustCompile(`@Entity\b`)
	reClassName           = regexp.MustCompile(`\b(?:class|record)\s+(\w+)`)
	reSwaggerImport       = regexp.MustCompile(`^\s*import\s+io\.swagger\.`)
	reImplSerializable    = regexp.MustCompile(`\bimplements\b[^{]*\bSerializable\b`)
	reForEachDeleteRef    = regexp.MustCompile(`\.forEach\(\s*\w+\s*::\s*delete\w*\b`)
	reForEachDeleteLambda = regexp.MustCompile(`\.forEach\(\s*\(?\s*\w+\s*\)?\s*->\s*\w+\.delete\w*\s*\(`)
	reNestedCollection    = regexp.MustCompile(`\b(?:List|Set|Collection)\s*<\s*(?:List|Set|Collection)\s*<`)
	reTestsClass          = regexp.MustCompile(`\bclass\s+\w+Tests\b`)
	reRequiredAttr        = regexp.MustCompile(`\brequired\b\s*=`)
)

// rules is the deterministic rule set. Every rule blocks by default
// (SeverityError → non-zero exit): greve is opt-in, so if you run it, a miss
// lights up hard rather than scrolling past as an advisory. A rule earns its
// place here only by being airtight; anything that can't be made precise stays
// a reviewer-tier lead in the standards corpus instead of shipping as a soft
// warning. Severity is still overridable per repo via .greve-review.yml for the
// rare tool-bug false positive (which is really a bug report against greve).
// Each rule is lexical — it runs over comment/string-stripped lines, so tokens
// inside comments or string literals never false-positive.
var rules = []rule{
	{
		ID: "no-zalando-problem", CorpusID: "err-problem-dept44", Default: SeverityError,
		Title: "Use dept44 Problem, never org.zalando.problem",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, reZalando, "imports org.zalando.problem — not on the classpath in dept44 8+; use se.sundsvall.dept44.problem.Problem", "")
		},
	},
	{
		ID: "no-wildcard-import", CorpusID: "imports-explicit", Default: SeverityError,
		Title: "No wildcard imports",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, reWildcard, "wildcard import — dept44 requires explicit imports", "")
		},
	},
	{
		ID: "no-lombok", CorpusID: "no-lombok", Default: SeverityError,
		Title: "No Lombok",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, reLombok, "Lombok is banned — write getters/setters/constructors/builders explicitly", "")
		},
	},
	{
		ID: "no-spring-scheduled", CorpusID: "scheduler-dept44scheduled", Default: SeverityError,
		Title: "Use @Dept44Scheduled, not @Scheduled",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, reScheduled, "Spring @Scheduled — use dept44 @Dept44Scheduled", "@Dept44Scheduled(name = \"...\", cron = \"...\", lockAtMostFor = \"...\")")
		},
	},
	{
		ID: "no-enum-in-api-model", CorpusID: "api-no-enums", Default: SeverityError,
		Title: "No enums in API model layer",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if !strings.Contains(f.Path, "/api/model/") {
				return nil
			}
			return lineHits(f, reEnumDecl, "enum in api/model — API models use String + @MemberOf/@OneOf + @Schema(allowableValues=...); enums belong in entities/internal", "")
		},
	},
	{
		ID: "circuitbreaker-on-feign", CorpusID: "resilience-circuitbreaker", Default: SeverityError,
		Title: "@FeignClient requires @CircuitBreaker",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if !strings.Contains(f.sanText, "@FeignClient") || strings.Contains(f.sanText, "@CircuitBreaker") {
				return nil
			}
			return []hit{{Line: firstMatchLine(f, reFeignAnno), Message: "@FeignClient without @CircuitBreaker", Fix: "@CircuitBreaker(name = \"<clientId>\")"}}
		},
	},
	{
		ID: "circuitbreaker-on-repository", CorpusID: "resilience-circuitbreaker", Default: SeverityError,
		Title: "Spring Data repository requires @CircuitBreaker",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if !reRepoIface.MatchString(f.sanText) || strings.Contains(f.sanText, "@CircuitBreaker") {
				return nil
			}
			return []hit{{Line: firstMatchLine(f, reRepoIface), Message: "Spring Data repository without @CircuitBreaker", Fix: "@CircuitBreaker(name = \"<repositoryName>\")"}}
		},
	},
	{
		ID: "no-field-injection", CorpusID: "constructor-injection", Default: SeverityError,
		Title: "Constructor injection, not @Autowired fields",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, reAutowired, "@Autowired — dept44 uses constructor injection with final dependencies (no field/setter injection)", "")
		},
	},
	{
		ID: "enumerated-string", CorpusID: "entity-enumerated-string", Default: SeverityError,
		Title: "@Enumerated must be EnumType.STRING",
		check: func(f *javaFile, _ *scanCtx) []hit {
			var hits []hit
			for i, s := range f.Sanitized {
				// Pass on STRING whether qualified (EnumType.STRING) or
				// static-imported (STRING); flag bare @Enumerated (defaults to
				// ORDINAL) and explicit ORDINAL.
				if strings.Contains(s, "@Enumerated") && !strings.Contains(s, "STRING") {
					hits = append(hits, hit{Line: i + 1, Message: "@Enumerated not set to STRING — store enums as varchar, never ordinal int", Fix: "@Enumerated(EnumType.STRING)"})
				}
			}
			return hits
		},
	},
	{
		ID: "missing-failure-test", CorpusID: "test-resource-failuretest", Default: SeverityError,
		Title: "Every {Resource}Test needs a {Resource}FailureTest",
		check: func(f *javaFile, ctx *scanCtx) []hit {
			if !strings.HasSuffix(f.base(), "Resource.java") {
				return nil
			}
			cls := strings.TrimSuffix(f.base(), ".java")
			if ctx.TestClasses[cls+"FailureTest"] {
				return nil
			}
			return []hit{{Line: f.classDeclLine(), Message: cls + " has no " + cls + "FailureTest — dept44 requires a {Resource}Test (happy path) + {Resource}FailureTest (validation) pair", Fix: ""}}
		},
	},
	{
		ID: "no-ternary", CorpusID: "no-ternary", Default: SeverityError,
		Title: "No ternary operator",
		check: func(f *javaFile, _ *scanCtx) []hit {
			var hits []hit
			for i, s := range f.Sanitized {
				if looksLikeTernary(s) {
					hits = append(hits, hit{Line: i + 1, Message: "ternary (?:) — dept44 bans it; use if/else, a helper, or Optional.map/.orElse", Fix: ""})
				}
			}
			return hits
		},
	},
	{
		ID: "no-public-resource", CorpusID: "resource-package-private", Default: SeverityError,
		Title: "Resource classes are package-private",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if !strings.HasSuffix(f.base(), "Resource.java") {
				return nil
			}
			return lineHits(f, rePublicRes, "Resource class should be package-private — drop 'public'", "")
		},
	},
	{
		ID: "pathvariable-redundant-name", CorpusID: "pathvariable-name", Default: SeverityError,
		Title: "No redundant @PathVariable name",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, rePathVarArg, "redundant @PathVariable name — rely on the parameter name (compiled with -parameters)", "")
		},
	},
	{
		ID: "unqualified-constant", CorpusID: "static-import-constants", Default: SeverityError,
		Title: "Static-import HttpStatus constants",
		check: func(f *javaFile, _ *scanCtx) []hit {
			var hits []hit
			for i, s := range f.Sanitized {
				if reImportLine.MatchString(s) {
					continue // the static import is the correct form, not a violation
				}
				if reQualStatus.MatchString(s) {
					hits = append(hits, hit{Line: i + 1, Message: "qualified HttpStatus.X — static-import the constant (write NOT_FOUND, not HttpStatus.NOT_FOUND)", Fix: ""})
				}
			}
			return hits
		},
	},
	{
		ID: "static-import-assertj", CorpusID: "static-import-assertions", Default: SeverityError,
		Title: "Static-import AssertJ Assertions", Test: true,
		check: func(f *javaFile, _ *scanCtx) []hit {
			var hits []hit
			for i, s := range f.Sanitized {
				if reImportLine.MatchString(s) {
					continue // the static import is the correct form, not a violation
				}
				if reFqnAssertj.MatchString(s) {
					hits = append(hits, hit{Line: i + 1, Message: "fully-qualified AssertJ helper (Assertions.* / Tuple.tuple) — static-import it (write assertThat / tuple, not the full package path)", Fix: ""})
				}
				if reFqnHamcrest.MatchString(s) {
					hits = append(hits, hit{Line: i + 1, Message: "fully-qualified org.hamcrest.MatcherAssert.assertThat — class-import MatcherAssert and write MatcherAssert.assertThat (so the bare AssertJ assertThat static import stays unambiguous)", Fix: ""})
				}
			}
			return hits
		},
	},
	{
		ID: "entity-column-length-mismatch", CorpusID: "entity-column-length", Default: SeverityError,
		Title: "Entity @Column length matches the DB migration",
		check: func(f *javaFile, ctx *scanCtx) []hit {
			if ctx.MigrationColumns == nil || !strings.HasSuffix(f.base(), "Entity.java") {
				return nil
			}
			// Use raw lines, not Sanitized: the sanitizer blanks string-literal contents, which would
			// strip the very column/table names this rule needs.
			tm := reTableName.FindStringSubmatch(strings.Join(f.Lines, "\n"))
			if tm == nil {
				return nil
			}
			cols := ctx.MigrationColumns[tm[1]]
			if cols == nil {
				return nil
			}
			var hits []hit
			for i, s := range f.Lines {
				trimmed := strings.TrimSpace(s)
				if !strings.Contains(s, "@Column") || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
					continue
				}
				nm := reColName.FindStringSubmatch(s)
				lm := reColLength.FindStringSubmatch(s)
				if nm == nil || lm == nil {
					continue
				}
				vm := reVarcharLen.FindStringSubmatch(cols[nm[1]])
				if vm == nil {
					continue
				}
				entLen, _ := strconv.Atoi(lm[1])
				migLen, _ := strconv.Atoi(vm[1])
				if entLen != migLen {
					hits = append(hits, hit{Line: i + 1, Message: fmt.Sprintf(
						"entity @Column length %d for '%s' != migration varchar(%d) — a value within the entity/@Size limit would be truncated at insert", entLen, nm[1], migLen), Fix: ""})
				}
			}
			return hits
		},
	},
	{
		ID: "prefer-tolist", CorpusID: "mined-tolist", Default: SeverityError,
		Title: "Use .toList(), not .collect(Collectors.toList())",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, reCollectToList, ".collect(Collectors.toList()) — use .toList() (returns an immutable list)", ".toList()")
		},
	},
	{
		ID: "no-jakarta-transactional", CorpusID: "mined-spring-transactional", Default: SeverityError,
		Title: "Use Spring @Transactional, not the Jakarta variant",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, reJakartaTx, "jakarta.transaction.Transactional — dept44 uses Spring's org.springframework.transaction.annotation.Transactional", "")
		},
	},
	{
		ID: "no-swagger-on-feign", CorpusID: "mined-no-swagger-on-feign", Default: SeverityError,
		Title: "No Swagger annotations on Feign clients",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if !strings.Contains(f.sanText, "@FeignClient") {
				return nil
			}
			return lineHits(f, reSwaggerImport, "Swagger/OpenAPI import in a @FeignClient interface — Feign clients produce no docs; annotate the REST controller instead", "")
		},
	},
	{
		ID: "serializable-needs-serialversionuid", CorpusID: "mined-serializable-needs-reason", Default: SeverityError,
		Title: "Serializable requires a serialVersionUID",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if strings.Contains(f.sanText, "serialVersionUID") {
				return nil
			}
			return lineHits(f, reImplSerializable, "implements Serializable without a serialVersionUID — declare one, or drop Serializable if it isn't needed", "private static final long serialVersionUID = 1L;")
		},
	},
	{
		ID: "no-controller-naming", CorpusID: "resource-naming", Default: SeverityError,
		Title: "REST controllers are named {Entity}Resource",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if !strings.Contains(f.Path, "/api/") && !reRestCtrlAnno.MatchString(f.sanText) {
				return nil
			}
			return lineHits(f, reController, "class named *Controller — dept44 REST controllers are named {Entity}Resource", "")
		},
	},
	{
		ID: "entity-suffix", CorpusID: "mined-entity-suffix", Default: SeverityError,
		Title: "@Entity classes are suffixed 'Entity'",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if !reEntityAnno.MatchString(f.sanText) {
				return nil
			}
			m := reClassName.FindStringSubmatch(f.sanText)
			if m == nil || strings.HasSuffix(m[1], "Entity") {
				return nil
			}
			return []hit{{Line: f.classDeclLine(), Message: "@Entity class '" + m[1] + "' is not suffixed 'Entity' — rename to " + m[1] + "Entity", Fix: ""}}
		},
	},
	{
		ID: "entity-equals-hashcode", CorpusID: "mined-entity-equals-hashcode", Default: SeverityError,
		Title: "@Entity classes implement equals & hashCode",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if f.Layer != LayerEntity || !reEntityAnno.MatchString(f.sanText) || strings.Contains(f.sanText, "@MappedSuperclass") {
				return nil
			}
			var decl string
			for _, s := range f.Sanitized {
				if reTypeDecl.MatchString(s) {
					decl = s
					break
				}
			}
			// Skip records (equals/hashCode are generated) and abstract or
			// inheriting entities (they may inherit equals/hashCode from a base).
			if strings.Contains(decl, "record ") || strings.Contains(decl, "abstract ") || strings.Contains(decl, " extends ") {
				return nil
			}
			if strings.Contains(f.sanText, "boolean equals(") && strings.Contains(f.sanText, "int hashCode(") {
				return nil
			}
			return []hit{{Line: f.classDeclLine(), Message: "@Entity class without equals & hashCode — dept44 entities implement both (BeanMatchers testBean expects them); exclude large blob/text fields", Fix: ""}}
		},
	},
	{
		ID: "deleteall-not-foreach", CorpusID: "mined-deleteall-not-foreach", Default: SeverityError,
		Title: "Use deleteAll(entities), not forEach(repo::delete)",
		check: func(f *javaFile, _ *scanCtx) []hit {
			var hits []hit
			for i, s := range f.Sanitized {
				if reForEachDeleteRef.MatchString(s) || reForEachDeleteLambda.MatchString(s) {
					hits = append(hits, hit{Line: i + 1, Message: "forEach(repo::delete) — use repository.deleteAll(entities): one call, cascades still fire through the persistence context", Fix: "repository.deleteAll(entities)"})
				}
			}
			return hits
		},
	},
	{
		ID: "no-nested-collections", CorpusID: "mined-no-nested-collections", Default: SeverityError,
		Title: "No nested collections in API models",
		check: func(f *javaFile, _ *scanCtx) []hit {
			if f.Layer != LayerPojo {
				return nil
			}
			return lineHits(f, reNestedCollection, "nested collection (e.g. List<List<...>>) in an API model — wrap the inner collection in a named type", "")
		},
	},
	{
		ID: "schema-required-mode", CorpusID: "mined-schema-required-mode", Default: SeverityError,
		Title: "Use @Schema(requiredMode=...), not the deprecated required=",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return schemaRequiredHits(f)
		},
	},
	{
		ID: "test-class-plural-suffix", CorpusID: "mined-test-class-singular", Default: SeverityError, Test: true,
		Title: "Test classes use the singular 'Test' suffix",
		check: func(f *javaFile, _ *scanCtx) []hit {
			return lineHits(f, reTestsClass, "test class named *Tests — dept44 uses the singular *Test suffix", "")
		},
	},
}

// schemaRequiredHits flags the deprecated @Schema(required = …) attribute
// (replaced by requiredMode) while ignoring the still-valid required= on
// @RequestParam/@RequestHeader/@RequestBody. It tracks @Schema(...) spans across
// lines by paren depth so multi-line annotations are covered, and only reports
// a required= that falls inside a @Schema span.
func schemaRequiredHits(f *javaFile) []hit {
	var hits []hit
	depth := 0
	for i, s := range f.Sanitized {
		scan := s
		if depth == 0 {
			idx := strings.Index(s, "@Schema")
			if idx < 0 {
				continue
			}
			scan = s[idx:]
			depth = parenDelta(scan)
		} else {
			depth += parenDelta(scan)
		}
		if reRequiredAttr.MatchString(scan) {
			hits = append(hits, hit{Line: i + 1, Message: "@Schema(required = …) is deprecated — use requiredMode = Schema.RequiredMode.REQUIRED with a matching @NotBlank/@NotNull", Fix: "requiredMode = Schema.RequiredMode.REQUIRED"})
		}
		if depth < 0 {
			depth = 0
		}
	}
	return hits
}

// parenDelta returns the count of '(' minus ')' in s.
func parenDelta(s string) int {
	d := 0
	for _, c := range s {
		switch c {
		case '(':
			d++
		case ')':
			d--
		}
	}
	return d
}

// RuleRefs maps each linter rule id to the standards-corpus id it enforces.
// The standards package asserts every referenced corpus id exists.
func RuleRefs() map[string]string {
	m := make(map[string]string, len(rules))
	for _, r := range rules {
		m[r.ID] = r.CorpusID
	}
	return m
}

// lineHits returns one hit per sanitized line matching re.
func lineHits(f *javaFile, re *regexp.Regexp, msg, fix string) []hit {
	var hits []hit
	for i, s := range f.Sanitized {
		if re.MatchString(s) {
			hits = append(hits, hit{Line: i + 1, Message: msg, Fix: fix})
		}
	}
	return hits
}

// firstMatchLine returns the 1-based line of the first sanitized match, or the
// type declaration line as a fallback anchor.
func firstMatchLine(f *javaFile, re *regexp.Regexp) int {
	for i, s := range f.Sanitized {
		if re.MatchString(s) {
			return i + 1
		}
	}
	return f.classDeclLine()
}

// looksLikeTernary heuristically detects a single-line ternary: a '?' (not a
// generic wildcard) followed later on the line by a ':' that is not part of
// '::'. Generic wildcards (<?, ? extends, ? super, ?>) are stripped first.
func looksLikeTernary(s string) bool {
	t := reGenericWld.ReplaceAllString(s, " ")
	q := strings.IndexByte(t, '?')
	if q < 0 {
		return false
	}
	rest := t[q+1:]
	for j := 0; j < len(rest); j++ {
		if rest[j] != ':' {
			continue
		}
		if j+1 < len(rest) && rest[j+1] == ':' { // "::" method ref
			j++
			continue
		}
		if j > 0 && rest[j-1] == ':' {
			continue
		}
		return true
	}
	return false
}
