package scan

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"

	"github.com/CheeziCrew/greve/internal/catalog"
)

// Dept44ParentArtifact identifies the Maven parent that marks a repo as a
// dept44 microservice.
const Dept44ParentArtifact = "dept44-service-parent"

type pomFile struct {
	Parent struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	} `xml:"parent"`
	GroupID      string `xml:"groupId"`
	ArtifactID   string `xml:"artifactId"`
	Version      string `xml:"version"`
	Name         string `xml:"name"`
	Dependencies struct {
		Dependency []struct {
			GroupID    string `xml:"groupId"`
			ArtifactID string `xml:"artifactId"`
			Version    string `xml:"version"`
			Scope      string `xml:"scope"`
		} `xml:"dependency"`
	} `xml:"dependencies"`
}

// pomInfo is what the scanner needs from a pom.xml.
type pomInfo struct {
	parentArtifact string
	parentVersion  string
	groupID        string
	artifactID     string
	version        string
	dependencies   []catalog.Dependency
	inputSpecs     []string // basenames of openapi-generator inputSpec files
}

// inputSpecRe extracts openapi-generator <inputSpec> values. The plugin
// configuration is free-form XML, so a regexp over the raw bytes beats
// mapping the whole plugin section.
var inputSpecRe = regexp.MustCompile(`<inputSpec>([^<]+)</inputSpec>`)

func parsePom(path string) (*pomInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pom pomFile
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, err
	}

	info := &pomInfo{
		parentArtifact: pom.Parent.ArtifactID,
		parentVersion:  pom.Parent.Version,
		groupID:        pom.GroupID,
		artifactID:     pom.ArtifactID,
		version:        pom.Version,
	}
	if info.groupID == "" {
		info.groupID = pom.Parent.GroupID
	}

	for _, d := range pom.Dependencies.Dependency {
		info.dependencies = append(info.dependencies, catalog.Dependency{
			GroupID:    d.GroupID,
			ArtifactID: d.ArtifactID,
			Version:    d.Version,
			Scope:      d.Scope,
		})
	}

	for _, m := range inputSpecRe.FindAllSubmatch(data, -1) {
		info.inputSpecs = append(info.inputSpecs, filepath.Base(string(m[1])))
	}

	return info, nil
}

// isDept44Service reports whether the pom declares the dept44 service parent.
func (p *pomInfo) isDept44Service() bool {
	return p.parentArtifact == Dept44ParentArtifact
}
