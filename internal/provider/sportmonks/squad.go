package sportmonks

import (
	"fmt"
	"strconv"
	"time"

	"github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/provider"
)

// catalogRoles maps Sportmonks position types to the P/D/C/A catalog. The
// provider gives one position per player, so no player gets a dual role here.
var catalogRoles = map[int]domain.CatalogRole{
	positionGoalkeeper: domain.CatalogGoalkeeper,
	positionDefender:   domain.CatalogDefender,
	positionMidfielder: domain.CatalogMidfielder,
	positionAttacker:   domain.CatalogAttacker,
}

// ParseSquad parses GET /squads/seasons/{season}/teams/{team}?include=player.
// A member is listed while the squad entry has no end date or its end date has
// not passed at the time of the payload.
func ParseSquad(r provider.Raw) (provider.Squad, error) {
	rows, err := decode[[]struct {
		PlayerID   id      `json:"player_id"`
		PositionID *int    `json:"position_id"`
		End        *string `json:"end"`
		Player     *struct {
			Name        string  `json:"name"`
			DisplayName string  `json:"display_name"`
			DateOfBirth *string `json:"date_of_birth"`
		} `json:"player"`
	}](r)
	if err != nil {
		return provider.Squad{}, err
	}
	var out provider.Squad
	for _, row := range rows {
		ref := row.PlayerID.String()
		if ref == "" {
			continue
		}
		m := provider.SquadMember{PlayerRef: ref, Listed: true}
		if row.PositionID != nil {
			m.Role = catalogRoles[*row.PositionID]
		}
		if row.End != nil && *row.End != "" {
			end, err := time.Parse(time.DateOnly, *row.End)
			if err != nil {
				return provider.Squad{}, fmt.Errorf("%w (%s): end %q: %w", ErrParse, r.Key, *row.End, err)
			}
			m.Listed = !end.Before(r.FetchedAt.Truncate(24 * time.Hour))
		}
		if p := row.Player; p != nil {
			m.Name = p.DisplayName
			if m.Name == "" {
				m.Name = p.Name
			}
			if p.DateOfBirth != nil && len(*p.DateOfBirth) >= 4 {
				if y, err := strconv.Atoi((*p.DateOfBirth)[:4]); err == nil && y > 1900 {
					m.BirthYear = &y
				}
			}
		}
		out.Members = append(out.Members, m)
	}
	return out, nil
}

// ParseSquad implements provider.Provider.
func (c *Client) ParseSquad(r provider.Raw) (provider.Squad, error) { return ParseSquad(r) }
