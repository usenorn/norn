package entity

type Location struct {
	CountryCode string
	City        string
}

func (l Location) Known() bool {
	return l.CountryCode != "" || l.City != ""
}
