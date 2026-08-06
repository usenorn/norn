package linear

const scopesQuery = `query ImportScopes($first: Int!) {
  teams(first: $first) {
    nodes { id key name }
  }
}`

const teamsQuery = `query ImportTeams($teams: [ID!]!, $first: Int!, $after: String) {
  teams(first: $first, after: $after, filter: { id: { in: $teams } }) {
    nodes { id key name createdAt updatedAt }
    pageInfo { hasNextPage endCursor }
  }
}`

const workflowStatesQuery = `query ImportWorkflowStates($teams: [ID!]!, $first: Int!, $after: String) {
  workflowStates(
    first: $first
    after: $after
    filter: { team: { id: { in: $teams } } }
  ) {
    nodes { id name type createdAt updatedAt team { id } }
    pageInfo { hasNextPage endCursor }
  }
}`

// labelsQuery serves both label phases. Linear models a label group as a label carrying
// children rather than as a type of its own, so the group and the label arrive together and
// are told apart on isGroup; asking twice with a filter on that flag would depend on a
// filter the schema does not document. A label with no team is a workspace label, which the
// issues of a chosen team may still carry, so it is narrowed alongside them rather than out.
const labelsQuery = `query ImportLabels($teams: [ID!]!, $first: Int!, $after: String) {
  issueLabels(
    first: $first
    after: $after
    filter: { or: [{ team: { id: { in: $teams } } }, { team: { null: true } }] }
  ) {
    nodes { id name color isGroup createdAt updatedAt team { id } parent { id } }
    pageInfo { hasNextPage endCursor }
  }
}`

const projectsQuery = `query ImportProjects($teams: [ID!]!, $first: Int!, $after: String) {
  projects(
    first: $first
    after: $after
    filter: { accessibleTeams: { some: { id: { in: $teams } } } }
  ) {
    nodes {
      id name slugId description createdAt updatedAt
      lead { id name displayName email }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const cyclesQuery = `query ImportCycles($teams: [ID!]!, $first: Int!, $after: String) {
  cycles(first: $first, after: $after, filter: { team: { id: { in: $teams } } }) {
    nodes { id number name startsAt endsAt completedAt createdAt updatedAt team { id } }
    pageInfo { hasNextPage endCursor }
  }
}`

const issuesQuery = `query ImportIssues($teams: [ID!]!, $first: Int!, $after: String) {
  issues(
    first: $first
    after: $after
    includeArchived: true
    filter: { team: { id: { in: $teams } } }
  ) {
    nodes {
      id title description priority estimate dueDate createdAt updatedAt
      team { id }
      state { id }
      project { id }
      cycle { id }
      parent { id }
      assignee { id name displayName email }
      creator { id name displayName email }
      labels(first: 250) { nodes { id } pageInfo { hasNextPage } }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// issueRelationsQuery walks issues rather than relations because Linear's root relation
// query takes no filter, and a run that has chosen three teams must not read the rest of the
// workspace. Both sides are asked for: a pair is only fully known at whichever of the two
// issues the relation was recorded on, and a pair split across two pages would otherwise
// have each page pick a different winner.
const issueRelationsQuery = `query ImportIssueRelations($teams: [ID!]!, $first: Int!, $after: String) {
  issues(
    first: $first
    after: $after
    includeArchived: true
    filter: { team: { id: { in: $teams } } }
  ) {
    nodes {
      id
      relations(first: 250) {
        nodes { id type createdAt updatedAt issue { id } relatedIssue { id } }
        pageInfo { hasNextPage }
      }
      inverseRelations(first: 250) {
        nodes { id type createdAt updatedAt issue { id } relatedIssue { id } }
        pageInfo { hasNextPage }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const commentsQuery = `query ImportComments($teams: [ID!]!, $first: Int!, $after: String) {
  comments(
    first: $first
    after: $after
    filter: { issue: { team: { id: { in: $teams } } } }
  ) {
    nodes {
      id body createdAt updatedAt
      issue { id }
      parent { id }
      user { id name displayName email }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// attachmentsQuery walks issues because a body's inline images and the files hanging off the
// row are one page of work: the marker written into a description and the file record it
// points at have to be derived from the same read, or the two disagree about what the image
// is called.
const attachmentsQuery = `query ImportAttachments($teams: [ID!]!, $first: Int!, $after: String) {
  issues(
    first: $first
    after: $after
    includeArchived: true
    filter: { team: { id: { in: $teams } } }
  ) {
    nodes {
      id description createdAt updatedAt
      attachments(first: 250) {
        nodes { id title subtitle url createdAt updatedAt }
        pageInfo { hasNextPage }
      }
      comments(first: 250) {
        nodes { id body createdAt updatedAt }
        pageInfo { hasNextPage }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`
