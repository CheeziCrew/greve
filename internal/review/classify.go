package review

import (
	"path"
	"strings"
)

// Layer is the dept44 component type a file belongs to, inferred from its path
// and name. Used to gate layer-specific rules and to label findings (and to
// cross-reference the standards corpus by category).
type Layer string

const (
	LayerResource    Layer = "resource"
	LayerPojo        Layer = "pojo"
	LayerEntity      Layer = "entity"
	LayerRepository  Layer = "repository"
	LayerMapper      Layer = "mapper"
	LayerIntegration Layer = "integration"
	LayerScheduler   Layer = "scheduler"
	LayerService     Layer = "service"
	LayerValidation  Layer = "validation"
	LayerGeneral     Layer = "general"
)

// ClassifyPath returns the dept44 component layer for a Java file path as a
// string. Exported for the standards corpus, which categorises rules by the
// same layer names.
func ClassifyPath(p string) string { return string(classify(p)) }

// classify maps a repo-relative, forward-slash Java path to a Layer. Order
// matters: the most specific package/name hints win. It mirrors the path hints
// already used by greve's pattern finder (internal/insight/patterns.go).
func classify(p string) Layer {
	base := path.Base(p)
	switch {
	case strings.Contains(p, "/api/model/"):
		return LayerPojo
	case strings.Contains(p, "/api/validation/"):
		return LayerValidation
	case strings.HasSuffix(base, "Resource.java"):
		return LayerResource
	case strings.Contains(p, "/integration/db/model/") || strings.HasSuffix(base, "Entity.java"):
		return LayerEntity
	case strings.HasSuffix(base, "Repository.java"):
		return LayerRepository
	case strings.Contains(p, "/service/mapper/") || strings.HasSuffix(base, "Mapper.java"):
		return LayerMapper
	case strings.Contains(p, "/service/scheduler/") || strings.HasSuffix(base, "Scheduler.java") || strings.HasSuffix(base, "Worker.java"):
		return LayerScheduler
	case strings.Contains(p, "/integration/"):
		return LayerIntegration
	case strings.Contains(p, "/service/"):
		return LayerService
	default:
		return LayerGeneral
	}
}
