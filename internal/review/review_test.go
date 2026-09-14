package review

import "testing"

// fireRules runs every rule over one in-memory file and returns how many times
// each rule id fired.
func fireRules(path, src string, ctx *scanCtx) map[string]int {
	if ctx == nil {
		ctx = &scanCtx{TestClasses: map[string]bool{}}
	}
	jf := newJavaFile(path, []byte(src))
	got := map[string]int{}
	for i := range rules {
		r := &rules[i]
		for range r.check(jf, ctx) {
			got[r.ID]++
		}
	}
	return got
}

const mainPath = "src/main/java/se/sundsvall/demo/service/DemoService.java"

func TestSanitizerSuppressesCommentsAndStrings(t *testing.T) {
	cases := []struct {
		name string
		src  string
		rule string
		want bool
	}{
		{"real zalando import fires", "import org.zalando.problem.Problem;", "no-zalando-problem", true},
		{"commented zalando import suppressed", "// import org.zalando.problem.Problem;", "no-zalando-problem", false},
		{"zalando in string suppressed", `String s = "import org.zalando.problem";`, "no-zalando-problem", false},
		{"zalando in block comment suppressed", "/* import org.zalando.problem.X; */", "no-zalando-problem", false},
		{"ternary in string suppressed", `String s = "a ? b : c";`, "no-ternary", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fireRules(mainPath, c.src, nil)
			if (got[c.rule] > 0) != c.want {
				t.Fatalf("rule %s fired=%v, want %v (src=%q)", c.rule, got[c.rule] > 0, c.want, c.src)
			}
		})
	}
}

func TestTernaryPrecision(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"int x = cond ? a : b;", true},
		{"return flag ? one() : two();", true},
		{"Map<String, ?> m = null;", false},
		{"List<? extends Foo> l = null;", false},
		{"void f(Class<?> c) {}", false},
		{"for (Foo f : foos) { run(f); }", false},
		{"stream.map(Foo::bar).toList();", false},
		{"case OPEN: break;", false},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got := fireRules(mainPath, c.src, nil)
			if (got["no-ternary"] > 0) != c.want {
				t.Fatalf("no-ternary fired=%v, want %v (src=%q)", got["no-ternary"] > 0, c.want, c.src)
			}
		})
	}
}

func TestCircuitBreakerOnFeign(t *testing.T) {
	missing := `@FeignClient(name = "party")
interface PartyClient {}`
	present := `@FeignClient(name = "party")
@CircuitBreaker(name = "party")
interface PartyClient {}`
	if fireRules("src/main/java/se/x/integration/party/PartyClient.java", missing, nil)["circuitbreaker-on-feign"] == 0 {
		t.Fatal("expected circuitbreaker-on-feign to fire when @CircuitBreaker is absent")
	}
	if fireRules("src/main/java/se/x/integration/party/PartyClient.java", present, nil)["circuitbreaker-on-feign"] != 0 {
		t.Fatal("did not expect circuitbreaker-on-feign when @CircuitBreaker is present")
	}
}

func TestCircuitBreakerOnRepository(t *testing.T) {
	src := `@Repository
interface DemoRepository extends JpaRepository<DemoEntity, String> {}`
	if fireRules("src/main/java/se/x/integration/db/DemoRepository.java", src, nil)["circuitbreaker-on-repository"] == 0 {
		t.Fatal("expected circuitbreaker-on-repository to fire")
	}
	ok := `@CircuitBreaker(name = "demoRepository")
interface DemoRepository extends JpaRepository<DemoEntity, String> {}`
	if fireRules("src/main/java/se/x/integration/db/DemoRepository.java", ok, nil)["circuitbreaker-on-repository"] != 0 {
		t.Fatal("did not expect circuitbreaker-on-repository when present")
	}
}

func TestEnumInApiModelLayerGated(t *testing.T) {
	src := "public enum Status { OPEN, CLOSED }"
	if fireRules("src/main/java/se/x/api/model/Status.java", src, nil)["no-enum-in-api-model"] == 0 {
		t.Fatal("expected no-enum-in-api-model to fire in api/model")
	}
	if fireRules("src/main/java/se/x/integration/db/model/StatusEntity.java", src, nil)["no-enum-in-api-model"] != 0 {
		t.Fatal("did not expect no-enum-in-api-model outside api/model")
	}
}

func TestEnumeratedString(t *testing.T) {
	if fireRules(mainPath, "@Enumerated(EnumType.STRING)", nil)["enumerated-string"] != 0 {
		t.Fatal("EnumType.STRING enumerated should pass")
	}
	if fireRules(mainPath, "@Enumerated(STRING)", nil)["enumerated-string"] != 0 {
		t.Fatal("static-imported STRING enumerated should pass")
	}
	if fireRules(mainPath, "@Enumerated", nil)["enumerated-string"] == 0 {
		t.Fatal("bare @Enumerated should fire")
	}
	if fireRules(mainPath, "@Enumerated(EnumType.ORDINAL)", nil)["enumerated-string"] == 0 {
		t.Fatal("ORDINAL enumerated should fire")
	}
}

func TestUnqualifiedConstantSkipsImports(t *testing.T) {
	if fireRules(mainPath, "import static org.springframework.http.HttpStatus.NOT_FOUND;", nil)["unqualified-constant"] != 0 {
		t.Fatal("the static import line is the correct form and must not fire")
	}
	if fireRules(mainPath, "throw Problem.valueOf(HttpStatus.NOT_FOUND, msg);", nil)["unqualified-constant"] == 0 {
		t.Fatal("qualified HttpStatus usage should fire")
	}
}

func TestStaticImportAssertjRule(t *testing.T) {
	const testPath = "src/test/java/se/sundsvall/demo/api/model/DemoTest.java"
	if fireRules(testPath, "import static org.assertj.core.api.Assertions.assertThat;", nil)["static-import-assertj"] != 0 {
		t.Fatal("the AssertJ static import line is the correct form and must not fire")
	}
	if fireRules(testPath, "org.assertj.core.api.Assertions.assertThat(x).isEqualTo(y);", nil)["static-import-assertj"] == 0 {
		t.Fatal("fully-qualified AssertJ assertThat should fire")
	}
	if fireRules(testPath, "org.hamcrest.MatcherAssert.assertThat(Foo.class, allOf());", nil)["static-import-assertj"] == 0 {
		t.Fatal("full-package Hamcrest assertThat should fire — class-import MatcherAssert instead")
	}
	if fireRules(testPath, "MatcherAssert.assertThat(Foo.class, allOf());", nil)["static-import-assertj"] != 0 {
		t.Fatal("class-qualified MatcherAssert.assertThat (with a class import) is the correct form and must not fire")
	}
	if fireRules(testPath, "org.assertj.core.groups.Tuple.tuple(\"A\", \"B\");", nil)["static-import-assertj"] == 0 {
		t.Fatal("full-package Tuple.tuple should fire — static-import tuple instead")
	}
	if fireRules(testPath, "tuple(\"A\", \"B\");", nil)["static-import-assertj"] != 0 {
		t.Fatal("static-imported bare tuple(...) is the correct form and must not fire")
	}
}

func TestEntityColumnLengthMismatchRule(t *testing.T) {
	entity := "src/main/java/se/x/document/integration/db/model/DocumentEntity.java"
	ctx := &scanCtx{
		TestClasses:      map[string]bool{},
		MigrationColumns: map[string]map[string]string{"errand_document": {"document_type": "varchar(64)"}},
	}
	mismatch := "@Table(name = \"errand_document\")\nclass DocumentEntity {\n@Column(name = \"document_type\", nullable = false, length = 255)\n}"
	if fireRules(entity, mismatch, ctx)["entity-column-length-mismatch"] == 0 {
		t.Fatal("entity length 255 vs migration varchar(64) should fire")
	}
	match := "@Table(name = \"errand_document\")\nclass DocumentEntity {\n@Column(name = \"document_type\", length = 64)\n}"
	if fireRules(entity, match, ctx)["entity-column-length-mismatch"] != 0 {
		t.Fatal("matching entity length and migration varchar must not fire")
	}
	// No schema in ctx -> rule no-ops (e.g. unit tests that pass nil ctx)
	if fireRules(entity, mismatch, nil)["entity-column-length-mismatch"] != 0 {
		t.Fatal("rule must no-op when no migration schema is available")
	}
}

func TestSimpleBans(t *testing.T) {
	checks := []struct {
		rule string
		src  string
	}{
		{"no-lombok", "import lombok.Data;"},
		{"no-wildcard-import", "import java.util.*;"},
		{"no-spring-scheduled", "@Scheduled(cron = \"0 0 * * * *\")"},
		{"no-field-injection", "@Autowired private DemoService svc;"},
		{"unqualified-constant", "throw Problem.valueOf(HttpStatus.NOT_FOUND);"},
		{"pathvariable-redundant-name", "void f(@PathVariable(\"municipalityId\") String m) {}"},
	}
	for _, c := range checks {
		t.Run(c.rule, func(t *testing.T) {
			if fireRules(mainPath, c.src, nil)[c.rule] == 0 {
				t.Fatalf("expected %s to fire on %q", c.rule, c.src)
			}
		})
	}
	// @Dept44Scheduled must NOT trip the Spring-@Scheduled ban.
	if fireRules(mainPath, "@Dept44Scheduled(name = \"job\")", nil)["no-spring-scheduled"] != 0 {
		t.Fatal("@Dept44Scheduled must not fire no-spring-scheduled")
	}
}

func TestMissingFailureTest(t *testing.T) {
	res := "src/main/java/se/x/api/DemoResource.java"
	src := "class DemoResource {}"
	if fireRules(res, src, &scanCtx{TestClasses: map[string]bool{}})["missing-failure-test"] == 0 {
		t.Fatal("expected missing-failure-test when no FailureTest exists")
	}
	ctx := &scanCtx{TestClasses: map[string]bool{"DemoResourceFailureTest": true}}
	if fireRules(res, src, ctx)["missing-failure-test"] != 0 {
		t.Fatal("did not expect missing-failure-test when FailureTest exists")
	}
}

func TestPublicResource(t *testing.T) {
	res := "src/main/java/se/x/api/DemoResource.java"
	if fireRules(res, "public class DemoResource {}", nil)["no-public-resource"] == 0 {
		t.Fatal("expected no-public-resource on a public Resource")
	}
	if fireRules(res, "class DemoResource {}", nil)["no-public-resource"] != 0 {
		t.Fatal("package-private Resource should pass")
	}
}

// TestEveryRuleBlocks encodes the invariant that greve is opt-in and blocks
// hard: every deterministic rule defaults to SeverityError, so any finding
// makes greve review exit non-zero. A rule that can't be made airtight belongs
// in the reviewer-tier corpus, not here as a soft warning.
func TestEveryRuleBlocks(t *testing.T) {
	for _, r := range rules {
		if r.Default != SeverityError {
			t.Errorf("rule %q defaults to %q — every rule must block (SeverityError)", r.ID, r.Default)
		}
	}
}

func TestPreferToList(t *testing.T) {
	if fireRules(mainPath, "return stream.collect(Collectors.toList());", nil)["prefer-tolist"] == 0 {
		t.Fatal("expected prefer-tolist on .collect(Collectors.toList())")
	}
	if fireRules(mainPath, "return stream.collect(toList());", nil)["prefer-tolist"] == 0 {
		t.Fatal("expected prefer-tolist on static-imported toList()")
	}
	if fireRules(mainPath, "return stream.toList();", nil)["prefer-tolist"] != 0 {
		t.Fatal(".toList() is the correct form and must not fire")
	}
	if fireRules(mainPath, "return stream.collect(groupingBy(Foo::key));", nil)["prefer-tolist"] != 0 {
		t.Fatal("other collectors must not fire")
	}
}

func TestNoJakartaTransactional(t *testing.T) {
	if fireRules(mainPath, "import jakarta.transaction.Transactional;", nil)["no-jakarta-transactional"] == 0 {
		t.Fatal("expected no-jakarta-transactional on the jakarta import")
	}
	if fireRules(mainPath, "import org.springframework.transaction.annotation.Transactional;", nil)["no-jakarta-transactional"] != 0 {
		t.Fatal("the Spring Transactional import must not fire")
	}
}

func TestNoSwaggerOnFeign(t *testing.T) {
	feign := "src/main/java/se/x/integration/party/PartyClient.java"
	withSwagger := "@FeignClient(name = \"party\")\nimport io.swagger.v3.oas.annotations.Operation;\ninterface PartyClient {}"
	if fireRules(feign, withSwagger, nil)["no-swagger-on-feign"] == 0 {
		t.Fatal("expected no-swagger-on-feign when a Feign client imports swagger")
	}
	// A resource with swagger imports but no @FeignClient must not fire.
	res := "src/main/java/se/x/api/DemoResource.java"
	if fireRules(res, "import io.swagger.v3.oas.annotations.Operation;\nclass DemoResource {}", nil)["no-swagger-on-feign"] != 0 {
		t.Fatal("swagger on a non-Feign class must not fire")
	}
}

func TestSerializableNeedsSerialVersionUID(t *testing.T) {
	missing := "class DemoEntity implements Serializable {}"
	if fireRules(mainPath, missing, nil)["serializable-needs-serialversionuid"] == 0 {
		t.Fatal("expected serializable-needs-serialversionuid when the UID is absent")
	}
	present := "class DemoEntity implements Serializable {\nprivate static final long serialVersionUID = 1L;\n}"
	if fireRules(mainPath, present, nil)["serializable-needs-serialversionuid"] != 0 {
		t.Fatal("a declared serialVersionUID must not fire")
	}
	if fireRules(mainPath, "class Demo {}", nil)["serializable-needs-serialversionuid"] != 0 {
		t.Fatal("a non-Serializable class must not fire")
	}
}

func TestNoControllerNaming(t *testing.T) {
	api := "src/main/java/se/x/api/DemoController.java"
	if fireRules(api, "class DemoController {}", nil)["no-controller-naming"] == 0 {
		t.Fatal("expected no-controller-naming on a *Controller in api/")
	}
	if fireRules(api, "class DemoResource {}", nil)["no-controller-naming"] != 0 {
		t.Fatal("a *Resource must not fire")
	}
	// Outside api/ and without @RestController, do not fire (avoids flagging misc classes).
	other := "src/main/java/se/x/service/DemoController.java"
	if fireRules(other, "class DemoController {}", nil)["no-controller-naming"] != 0 {
		t.Fatal("a Controller-named class outside api/ without @RestController must not fire")
	}
	if fireRules(other, "@RestController\nclass DemoController {}", nil)["no-controller-naming"] == 0 {
		t.Fatal("a @RestController-annotated *Controller must fire even outside api/")
	}
}

func TestEntitySuffix(t *testing.T) {
	ent := "src/main/java/se/x/integration/db/model/Errand.java"
	if fireRules(ent, "@Entity\nclass Errand {}", nil)["entity-suffix"] == 0 {
		t.Fatal("expected entity-suffix on an @Entity class not ending in Entity")
	}
	if fireRules(ent, "@Entity\nclass ErrandEntity {}", nil)["entity-suffix"] != 0 {
		t.Fatal("a properly-suffixed @Entity must not fire")
	}
	// @EntityListeners must not be mistaken for @Entity.
	if fireRules(ent, "@EntityListeners(AuditListener.class)\nclass Errand {}", nil)["entity-suffix"] != 0 {
		t.Fatal("@EntityListeners must not trigger entity-suffix")
	}
}

func TestEntityEqualsHashCode(t *testing.T) {
	ent := "src/main/java/se/x/integration/db/model/DemoEntity.java"
	missing := "@Entity\nclass DemoEntity {\nprivate String id;\n}"
	if fireRules(ent, missing, nil)["entity-equals-hashcode"] == 0 {
		t.Fatal("expected entity-equals-hashcode when both are absent")
	}
	present := "@Entity\nclass DemoEntity {\npublic boolean equals(Object o) { return true; }\npublic int hashCode() { return 1; }\n}"
	if fireRules(ent, present, nil)["entity-equals-hashcode"] != 0 {
		t.Fatal("an entity with equals & hashCode must not fire")
	}
	// Inheriting entities may inherit equals/hashCode — do not fire.
	if fireRules(ent, "@Entity\nclass DemoEntity extends BaseEntity {}", nil)["entity-equals-hashcode"] != 0 {
		t.Fatal("an inheriting entity must not fire")
	}
	// A non-@Entity in the entity layer (e.g. a projection) must not fire.
	if fireRules(ent, "class DemoEntity {}", nil)["entity-equals-hashcode"] != 0 {
		t.Fatal("a class without @Entity must not fire")
	}
}

func TestDeleteAllNotForEach(t *testing.T) {
	if fireRules(mainPath, "found.forEach(repository::delete);", nil)["deleteall-not-foreach"] == 0 {
		t.Fatal("expected deleteall-not-foreach on forEach(repo::delete)")
	}
	if fireRules(mainPath, "found.forEach(e -> repository.delete(e));", nil)["deleteall-not-foreach"] == 0 {
		t.Fatal("expected deleteall-not-foreach on the lambda form")
	}
	if fireRules(mainPath, "repository.deleteAll(found);", nil)["deleteall-not-foreach"] != 0 {
		t.Fatal("deleteAll must not fire")
	}
	if fireRules(mainPath, "found.forEach(System.out::println);", nil)["deleteall-not-foreach"] != 0 {
		t.Fatal("a non-delete forEach must not fire")
	}
}

func TestNoNestedCollections(t *testing.T) {
	pojo := "src/main/java/se/x/api/model/FormSnapshotField.java"
	if fireRules(pojo, "List<List<FormSnapshotField>> groups;", nil)["no-nested-collections"] == 0 {
		t.Fatal("expected no-nested-collections on List<List<...>> in api/model")
	}
	if fireRules(pojo, "List<FormSnapshotGroup> groups;", nil)["no-nested-collections"] != 0 {
		t.Fatal("a flat list must not fire")
	}
	// Outside api/model the rule is not scoped to fire.
	svc := "src/main/java/se/x/service/DemoService.java"
	if fireRules(svc, "List<List<String>> tmp;", nil)["no-nested-collections"] != 0 {
		t.Fatal("nested collections outside api/model must not fire")
	}
}

func TestSchemaRequiredMode(t *testing.T) {
	pojo := "src/main/java/se/x/api/model/Demo.java"
	if fireRules(pojo, "@Schema(description = \"x\", required = true)\nString name;", nil)["schema-required-mode"] == 0 {
		t.Fatal("expected schema-required-mode on the deprecated required= attribute")
	}
	// Multi-line @Schema with required on its own line.
	multi := "@Schema(\ndescription = \"x\",\nrequired = true)\nString name;"
	if fireRules(pojo, multi, nil)["schema-required-mode"] == 0 {
		t.Fatal("expected schema-required-mode across a multi-line @Schema")
	}
	if fireRules(pojo, "@Schema(requiredMode = Schema.RequiredMode.REQUIRED)\nString name;", nil)["schema-required-mode"] != 0 {
		t.Fatal("requiredMode is the correct form and must not fire")
	}
	// The still-valid required= on @RequestParam/@RequestHeader must NOT fire.
	res := "src/main/java/se/x/api/DemoResource.java"
	if fireRules(res, "@RequestParam(required = false) String q", nil)["schema-required-mode"] != 0 {
		t.Fatal("required= on @RequestParam is valid and must not fire")
	}
}

func TestTestClassPluralSuffix(t *testing.T) {
	tst := "src/test/java/se/x/service/DemoServiceTests.java"
	if fireRules(tst, "class DemoServiceTests {}", nil)["test-class-plural-suffix"] == 0 {
		t.Fatal("expected test-class-plural-suffix on a *Tests class")
	}
	if fireRules("src/test/java/se/x/service/DemoServiceTest.java", "class DemoServiceTest {}", nil)["test-class-plural-suffix"] != 0 {
		t.Fatal("a singular *Test class must not fire")
	}
}
