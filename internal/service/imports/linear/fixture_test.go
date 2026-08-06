package linear_test

const (
	engineeringTeam = "team-8c2f4d"
	operationsTeam  = "team-51ba90"

	inflightState = "state-9d3a11"
	shippedState  = "state-4c8e02"

	areaGroup   = "labelgroup-2b71"
	parserLabel = "label-77fd"

	atlasProject = "project-atlas-1c9"

	openingCycle = "cycle-0aa1"
	closingCycle = "cycle-0aa2"

	hubIssue   = "issue-6d21"
	looseIssue = "issue-6d22"
	childIssue = "issue-6d23"

	firstComment = "comment-3f10"

	relatedLink   = "rel-1000-related"
	blockingLink  = "rel-2000-blocks"
	duplicateLink = "rel-3000-duplicate"
	unknownLink   = "rel-4000-similar"

	pullRequestRow = "attach-pr-8812"
	screenshotRow  = "attach-shot-8813"

	shotAddress = "https://uploads.linear.app/9f/2c/the%20failure.png"
	shotSigned  = shotAddress + "?signature=abcdef"
	pullRequest = "https://github.com/usenorn/norn/pull/812"
)

func wholeWorkspace() map[string]string {
	return map[string]string{
		"ImportScopes": `{"teams":{"nodes":[
			{"id":"` + engineeringTeam + `","key":"ENG","name":"Engineering"},
			{"id":"` + operationsTeam + `","key":"OPS","name":"Operations"}
		]}}`,

		"ImportTeams": `{"teams":{
			"nodes":[
				{"id":"` + engineeringTeam + `","key":"ENG","name":"Engineering",
				 "createdAt":"2025-11-03T09:12:00.000Z","updatedAt":"2026-01-19T11:00:00.000Z"},
				{"id":"` + operationsTeam + `","key":"OPS","name":"Operations",
				 "createdAt":"2025-11-03T09:14:00.000Z","updatedAt":"2026-01-19T11:00:00.000Z"}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-teams"}
		}}`,

		"ImportWorkflowStates": `{"workflowStates":{
			"nodes":[
				{"id":"` + inflightState + `","name":"In Progress","type":"started",
				 "createdAt":"2025-11-03T09:12:00.000Z","updatedAt":"2025-11-03T09:12:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"}},
				{"id":"` + shippedState + `","name":"Done","type":"completed",
				 "createdAt":"2025-11-03T09:12:00.000Z","updatedAt":"2025-11-03T09:12:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-states"}
		}}`,

		"ImportLabels": `{"issueLabels":{
			"nodes":[
				{"id":"` + areaGroup + `","name":"Area","color":"#4a5568","isGroup":true,
				 "createdAt":"2025-11-04T08:00:00.000Z","updatedAt":"2025-11-04T08:00:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"},"parent":null},
				{"id":"` + parserLabel + `","name":"parser","color":"#bb2222","isGroup":false,
				 "createdAt":"2025-11-04T08:01:00.000Z","updatedAt":"2025-11-04T08:01:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"},"parent":{"id":"` + areaGroup + `"}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-labels"}
		}}`,

		"ImportProjects": `{"projects":{
			"nodes":[
				{"id":"` + atlasProject + `","name":"Atlas","slugId":"atlas-9f2",
				 "description":"Rebuild the ingest path.",
				 "createdAt":"2025-12-01T10:00:00.000Z","updatedAt":"2026-01-08T10:00:00.000Z",
				 "lead":{"id":"user-rae","name":"Rae Okafor","displayName":"rae",
				         "email":"rae@northwind.co"}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-projects"}
		}}`,

		"ImportCycles": `{"cycles":{
			"nodes":[
				{"id":"` + openingCycle + `","number":41,"name":"Ingest",
				 "startsAt":"2026-01-05T00:00:00.000Z","endsAt":"2026-01-19T00:00:00.000Z",
				 "completedAt":"2026-01-19T00:00:00.000Z",
				 "createdAt":"2025-12-20T00:00:00.000Z","updatedAt":"2026-01-19T00:00:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"}},
				{"id":"` + closingCycle + `","number":42,"name":"Ingest II",
				 "startsAt":"2026-01-19T00:00:00.000Z","endsAt":"2026-02-02T00:00:00.000Z",
				 "completedAt":null,
				 "createdAt":"2026-01-05T00:00:00.000Z","updatedAt":"2026-01-19T00:00:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-cycles"}
		}}`,

		"ImportIssues": `{"issues":{
			"nodes":[
				{"id":"` + hubIssue + `","title":"Rework the hub",
				 "description":"` + hubDescription + `",
				 "priority":2,"estimate":3,"dueDate":"2026-02-10",
				 "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-18T16:30:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"},"state":{"id":"` + inflightState + `"},
				 "project":{"id":"` + atlasProject + `"},"cycle":{"id":"` + openingCycle + `"},
				 "parent":null,
				 "assignee":{"id":"user-rae","name":"Rae Okafor","displayName":"rae",
				             "email":"rae@northwind.co"},
				 "creator":{"id":"user-otto","name":"","displayName":"otto",
				            "email":"otto@northwind.co"},
				 "labels":{"nodes":[{"id":"` + parserLabel + `"}]}},
				{"id":"` + looseIssue + `","title":"Tidy the loose ends",
				 "description":"See the [runbook](https://example.com/runbook) and ![a diagram](https://example.com/a.png).",
				 "priority":0,"estimate":0,"dueDate":null,
				 "createdAt":"2026-01-07T09:00:00.000Z","updatedAt":"2026-01-07T09:00:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"},"state":{"id":"` + shippedState + `"},
				 "project":null,"cycle":null,"parent":null,
				 "assignee":null,"creator":null,
				 "labels":{"nodes":[]}},
				{"id":"` + childIssue + `","title":"Carve out the decoder",
				 "description":"","priority":4,"estimate":1,"dueDate":null,
				 "createdAt":"2026-01-08T09:00:00.000Z","updatedAt":"2026-01-08T09:00:00.000Z",
				 "team":{"id":"` + engineeringTeam + `"},"state":{"id":"` + inflightState + `"},
				 "project":null,"cycle":null,"parent":{"id":"` + hubIssue + `"},
				 "assignee":null,"creator":null,
				 "labels":{"nodes":[]}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-issues"}
		}}`,

		"ImportIssueRelations": `{"issues":{
			"nodes":[
				{"id":"` + hubIssue + `",
				 "relations":{"nodes":[
					{"id":"` + relatedLink + `","type":"related",
					 "createdAt":"2026-01-09T09:00:00.000Z","updatedAt":"2026-01-09T09:00:00.000Z",
					 "issue":{"id":"` + hubIssue + `"},"relatedIssue":{"id":"` + looseIssue + `"}},
					{"id":"` + blockingLink + `","type":"blocks",
					 "createdAt":"2026-01-09T09:05:00.000Z","updatedAt":"2026-01-09T09:05:00.000Z",
					 "issue":{"id":"` + hubIssue + `"},"relatedIssue":{"id":"` + looseIssue + `"}},
					{"id":"` + unknownLink + `","type":"similar",
					 "createdAt":"2026-01-09T09:07:00.000Z","updatedAt":"2026-01-09T09:07:00.000Z",
					 "issue":{"id":"` + hubIssue + `"},"relatedIssue":{"id":"` + childIssue + `"}}
				 ]},
				 "inverseRelations":{"nodes":[]}},
				{"id":"` + looseIssue + `",
				 "relations":{"nodes":[]},
				 "inverseRelations":{"nodes":[
					{"id":"` + duplicateLink + `","type":"duplicate",
					 "createdAt":"2026-01-09T09:06:00.000Z","updatedAt":"2026-01-09T09:06:00.000Z",
					 "issue":{"id":"` + looseIssue + `"},"relatedIssue":{"id":"` + hubIssue + `"}},
					{"id":"` + blockingLink + `","type":"blocks",
					 "createdAt":"2026-01-09T09:05:00.000Z","updatedAt":"2026-01-09T09:05:00.000Z",
					 "issue":{"id":"` + hubIssue + `"},"relatedIssue":{"id":"` + looseIssue + `"}}
				 ]}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-relations"}
		}}`,

		"ImportComments": `{"comments":{
			"nodes":[
				{"id":"` + firstComment + `","body":"The frame is dropped in the decoder.",
				 "createdAt":"2026-01-10T09:00:00.000Z","updatedAt":"2026-01-10T09:00:00.000Z",
				 "issue":{"id":"` + hubIssue + `"},"parent":null,
				 "user":{"id":"user-otto","name":"Otto Lindqvist","displayName":"otto",
				         "email":"otto@northwind.co"}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-comments"}
		}}`,

		"ImportAttachments": `{"issues":{
			"nodes":[
				{"id":"` + hubIssue + `",
				 "description":"` + hubDescription + `",
				 "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-18T16:30:00.000Z",
				 "attachments":{"nodes":[
					{"id":"` + pullRequestRow + `","title":"Fix the decoder",
					 "subtitle":"usenorn/norn#812","url":"` + pullRequest + `",
					 "createdAt":"2026-01-11T09:00:00.000Z","updatedAt":"2026-01-11T09:00:00.000Z"},
					{"id":"` + screenshotRow + `","title":"the failure.png",
					 "subtitle":"","url":"` + shotSigned + `",
					 "createdAt":"2026-01-11T09:02:00.000Z","updatedAt":"2026-01-11T09:02:00.000Z"}
				 ]},
				 "comments":{"nodes":[]}}
			],
			"pageInfo":{"hasNextPage":false,"endCursor":"cursor-attachments"}
		}}`,
	}
}
