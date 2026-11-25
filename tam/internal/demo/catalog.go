package demo

import "agile-suite/tam/internal/backend"

// demoDay formats a day relative to the dataset's base date the way Jira
// writes a version's dates.
func demoDay(days int) string {
	return base.AddDate(0, 0, days).Format("2006-01-02")
}

// Versions is the demo project's fix versions: one archived, one released
// with the closed sprint, one in progress with the active sprint, and one
// not yet planned. Each call builds a fresh slice.
func Versions(projectKey string) []backend.Version {
	return []backend.Version{
		{ID: 10100, Project: projectKey, Name: "v1.0", ReleaseDate: demoDay(-60), Released: true, Archived: true},
		{ID: 10101, Project: projectKey, Name: "v1.1", Description: "Promo codes", ReleaseDate: demoDay(-14), Released: true},
		{ID: 10102, Project: projectKey, Name: "v1.2", Description: "Checkout fixes", StartDate: demoDay(-7), ReleaseDate: demoDay(14)},
		{ID: 10103, Project: projectKey, Name: "v1.3"},
	}
}

// Components is the demo project's components, named after the areas the
// dataset's issues cover. Each call builds a fresh slice.
func Components(projectKey string) []backend.Component {
	return []backend.Component{
		{ID: 10000, Project: projectKey, Name: "Accounts", AssigneeType: "PROJECT_DEFAULT"},
		{ID: 10001, Project: projectKey, Name: "Checkout", Description: "Cart, payment and order summary", LeadName: "ranand", LeadDisplayName: "R. Anand", AssigneeType: "COMPONENT_LEAD"},
		{ID: 10002, Project: projectKey, Name: "Payments", AssigneeType: "PROJECT_DEFAULT"},
		{ID: 10003, Project: projectKey, Name: "Platform", AssigneeType: "PROJECT_DEFAULT"},
		{ID: 10004, Project: projectKey, Name: "Promotions", AssigneeType: "PROJECT_DEFAULT"},
	}
}

// tagCatalog files every issue under one or two components by its position,
// and gives it the fix version its sprint shipped or ships in: Sprint 11
// closed with v1.1, Sprint 12 is building v1.2, and half of the rest is
// planned for v1.3. Requirements are not released, so they get none. It
// runs after the dataset is built so it does not move the seeded random
// draws the rest of the dataset depends on.
func tagCatalog(issues []backend.Issue) {
	comps := []int{10000, 10001, 10002, 10003, 10004}
	for i := range issues {
		iss := &issues[i]
		iss.Components = []int{comps[i%len(comps)]}
		if i%3 == 0 {
			iss.Components = append(iss.Components, comps[(i+2)%len(comps)])
		}
		iss.FixVersions = []int{}
		switch {
		case iss.Type == backend.TypeRequirement:
		case iss.SprintID == "11":
			iss.FixVersions = []int{10101}
		case iss.SprintID == "12":
			iss.FixVersions = []int{10102}
		case i%2 == 0:
			iss.FixVersions = []int{10103}
		}
	}
}
