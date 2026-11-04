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

// ParseSquad parses GET /squads/seasons/{season}/teams/{team}?include=player. Its rows carry no
// start or end date, so they say who was in the squad during the season, not who still is.
func ParseSquad(r provider.Raw) (provider.Squad, error) {
	rows, err := decode[[]struct {
		PlayerID   id   `json:"player_id"`
		PositionID *int `json:"position_id"`
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
		m := provider.SquadMember{PlayerRef: ref}
		if row.PositionID != nil {
			m.Role = catalogRoles[*row.PositionID]
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

// ParseCurrentSquad parses GET /squads/teams/{team}. `end` is the end of the player's contract.
func ParseCurrentSquad(r provider.Raw) (provider.CurrentSquad, error) {
	rows, err := decode[[]struct {
		PlayerID id      `json:"player_id"`
		End      *string `json:"end"`
	}](r)
	if err != nil {
		return provider.CurrentSquad{}, err
	}
	var out provider.CurrentSquad
	for _, row := range rows {
		ref := row.PlayerID.String()
		if ref == "" {
			continue
		}
		m := provider.CurrentMember{PlayerRef: ref}
		if row.End != nil && *row.End != "" {
			end, err := time.Parse(time.DateOnly, *row.End)
			if err != nil {
				return provider.CurrentSquad{}, fmt.Errorf("%w (%s): end %q: %w", ErrParse, r.Key, *row.End, err)
			}
			m.Until = &end
		}
		out.Members = append(out.Members, m)
	}
	return out, nil
}

// ParseSquad implements provider.Provider.
func (c *Client) ParseSquad(r provider.Raw) (provider.Squad, error) { return ParseSquad(r) }

// ParseCurrentSquad implements provider.Provider.
func (c *Client) ParseCurrentSquad(r provider.Raw) (provider.CurrentSquad, error) {
	return ParseCurrentSquad(r)
}
