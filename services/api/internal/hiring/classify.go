package hiring

import (
	"regexp"
	"strings"
)

// TypeRule names the heuristic that set requirement.type on rows
// written before the extractor emits a type itself (step 5, FR-HF-02).
// It is recorded in the row's extraction provenance so a reader knows
// the type was inferred by code from the text, not stated by the
// model, and so rows typed by the model can be told apart later.
const TypeRule = "type-heuristic-v0"

var yearsPattern = regexp.MustCompile(`\b\d+\s*\+?\s*(?:-\s*\d+\s*)?(?:years?|yrs?)\b|\byears? of\b`)

// InferType guesses a requirement's type from its text and named
// parties, returning only codes in RequirementTypes. Order matters:
// a behavioral adjective wins over anything else because the model
// must never judge it (FR-HF-18); a credential wins over the years it
// may also mention; named parties make a relationship; a years clause
// makes experience; a known tool word makes a tool; an industry word a
// domain; a verb of achievement an outcome; the rest is a skill. Wrong
// often enough to be replaced, and honest about being a guess.
func InferType(text string, namedParties []string) string {
	t := " " + strings.ToLower(strings.TrimSpace(text)) + " "
	switch {
	case containsAny(t, behavioralMarkers):
		return "behavioral"
	case containsAny(t, credentialMarkers):
		return "credential"
	case len(namedParties) > 0 || containsAny(t, relationshipMarkers):
		return "relationship"
	case containsAny(t, constraintMarkers):
		return "constraint"
	case yearsPattern.MatchString(t):
		return "experience"
	case containsAny(t, toolMarkers):
		return "tool"
	case containsAny(t, domainMarkers):
		return "domain"
	case containsAny(t, outcomeMarkers):
		return "outcome"
	default:
		return "skill"
	}
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

var (
	behavioralMarkers = []string{
		"self-starter", "self starter", "team player", "communication skills", "interpersonal",
		"attitude", "passion", "passionate", "motivated", "collaborative", "detail-oriented",
		"detail oriented", "attention to detail", "work ethic", "fast-paced", "fast paced",
		"adaptable", "curious", "proactive", "ownership mentality", "problem-solver", "problem solver",
	}
	credentialMarkers = []string{
		"degree", "bachelor", "master's", "masters", "m.s.", "b.s.", "bs/ms", "b.sc", "m.sc", "mba",
		"phd", "ph.d", "doctorate", "certification", "certified", "certificate", "license", "licensed",
		"licensure", " pe ", "professional engineer", "diploma",
	}
	relationshipMarkers = []string{
		"partnership with", "relationship with", "relationships with", "vendor relationship",
		"working relationship", "customer relationship", "key accounts",
	}
	constraintMarkers = []string{
		"located in", "relocat", "on-site", "onsite", "on site", "travel", "clearance", "shift work",
		"night shift", "weekends", "authorized to work", "work authorization", "citizen", "remote",
		"hybrid", "driver's license", "drivers license", "physical requirement", "lift ",
	}
	toolMarkers = []string{
		" plc", "scada", " sql", "python", " go ", "golang", " java", " aws", "azure", "gcp",
		"ignition", "rockwell", "allen-bradley", "allen bradley", "siemens", "excel", " sap ", "sap ",
		"power bi", "tableau", " git", "docker", "kubernetes", "linux", "react", "typescript",
		"javascript", "c++", "c#", ".net", " mes ", " erp", " crm", "jira", "autocad", "solidworks",
		"matlab", "labview", "wonderware", "aveva", "studio 5000", "tia portal", "opc", "mqtt",
		"modbus", "ethernet/ip", "profinet", "salesforce", "workday", "servicenow", "snowflake",
		"spark", "kafka", "terraform", "ansible", "grafana", "prometheus", "postgres", "mysql",
		"oracle", "mongodb", "redis",
	}
	domainMarkers = []string{
		"manufacturing", "pharmaceutical", "food and beverage", "food & beverage", "aerospace",
		"automotive", "oil and gas", "oil & gas", "utilities", "utility", "healthcare", "hospital",
		"financial services", "banking", "insurance", "industry", "industrial", "regulated", "gmp",
		"fda", "iso ", "medical device", "defense", "defence", "mining", "chemical", "semiconductor",
		"consumer packaged", "cpg", "logistics", "warehouse", "distribution", "retail", "public sector",
	}
	outcomeMarkers = []string{
		"delivered", "deliver ", "led the", "reduced", "increased", "launched", "track record",
		"proven ability", "success in", "successfully", "grew", "scaled", "turned around",
		"achieved", "improved", "drove", "built and", "owned the",
	}
)
