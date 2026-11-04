package domain

// CatalogRole is a position in the league role catalog: portiere, difensore,
// centrocampista, attaccante. It is a player's listed position, unlike Role,
// which is what the player did in one match.
type CatalogRole string

// The role catalog.
const (
	CatalogGoalkeeper CatalogRole = "P"
	CatalogDefender   CatalogRole = "D"
	CatalogMidfielder CatalogRole = "C"
	CatalogAttacker   CatalogRole = "A"
)

// Valid reports whether r is in the catalog.
func (r CatalogRole) Valid() bool {
	switch r {
	case CatalogGoalkeeper, CatalogDefender, CatalogMidfielder, CatalogAttacker:
		return true
	}
	return false
}
