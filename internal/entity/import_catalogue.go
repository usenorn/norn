package entity

type ImportCatalogue struct {
	Scopes  []ImportScope
	Columns []ImportColumn
	Notes   []string
}

type ImportScope struct {
	Key    string
	Name   string
	Detail string
}

type ImportColumn struct {
	Index      int
	Header     string
	Proposed   string
	Confidence string
}
