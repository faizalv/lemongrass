package session

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	RolePilot   = "pilot"
	RoleCopilot = "copilot"

	// Every copilot loads this skill.
	CopilotSkill = "lgrass-copilot"
)

type Group struct {
	ID          int64
	PilotTabID  string
	ThreadID    int64
	Name        string
	CreatedAt   string
	DisbandedAt string // empty while the group is live
}

func (g Group) Live() bool { return g.DisbandedAt == "" }

type Member struct {
	TabID  string
	Role   string
	Label  string
	Vendor string
	Prompt string   // the assignment from the group config
	Skills []string // skills the member must load, beyond the ones every copilot loads
}

var (
	ErrNoSuchGroup   = errors.New("session: no such workgroup in this project")
	ErrAlreadyInside = errors.New("session: a tab can belong to only one live workgroup")
)

// Creates the group, its members, and its one thread in a single transaction. The pilot is added as a member.
func (s *Store) CreateGroup(name string, pilot Member, copilots []Member) (Group, error) {
	pilot.Role = RolePilot
	members := []Member{pilot}
	for _, c := range copilots {
		c.Role = RoleCopilot
		members = append(members, c)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback()

	for _, m := range members {
		var one int
		err := tx.QueryRow(`
			SELECT 1 FROM lg_group_members m JOIN lg_groups g ON g.id = m.group_id
			WHERE m.tab_id = ? AND g.project_id = ? AND g.disbanded_at IS NULL
		`, m.TabID, s.projectID).Scan(&one)
		if err == nil {
			return Group{}, fmt.Errorf("%w (%s)", ErrAlreadyInside, TabLabel(m.TabID))
		}
		if err != sql.ErrNoRows {
			return Group{}, err
		}
	}

	ts := now()
	res, err := tx.Exec(`INSERT INTO lg_groups (project_id, pilot_tab_id, created_at) VALUES (?, ?, ?)`, s.projectID, pilot.TabID, ts)
	if err != nil {
		return Group{}, err
	}
	groupID, err := res.LastInsertId()
	if err != nil {
		return Group{}, err
	}
	for _, m := range members {
		if _, err := tx.Exec(`INSERT INTO lg_group_members (group_id, tab_id, role, label, vendor, prompt, skills) VALUES (?, ?, ?, ?, ?, ?, ?)`, groupID, m.TabID, m.Role, m.Label, m.Vendor, m.Prompt, strings.Join(m.Skills, ",")); err != nil {
			return Group{}, err
		}
	}
	res, err = tx.Exec(`INSERT INTO lg_threads (project_id, title, group_id, created_by, created_at) VALUES (?, ?, ?, ?, ?)`, s.projectID, name, groupID, pilot.TabID, ts)
	if err != nil {
		return Group{}, err
	}
	threadID, err := res.LastInsertId()
	if err != nil {
		return Group{}, err
	}
	if err := tx.Commit(); err != nil {
		return Group{}, err
	}
	return Group{ID: groupID, PilotTabID: pilot.TabID, ThreadID: threadID, Name: name, CreatedAt: ts}, nil
}

const groupSelect = `
	SELECT g.id, g.pilot_tab_id, COALESCE(t.id, 0), COALESCE(t.title, ''), g.created_at, COALESCE(g.disbanded_at, '')
	FROM lg_groups g LEFT JOIN lg_threads t ON t.group_id = g.id
`

func scanGroup(row *sql.Row) (Group, error) {
	var g Group
	err := row.Scan(&g.ID, &g.PilotTabID, &g.ThreadID, &g.Name, &g.CreatedAt, &g.DisbandedAt)
	if err == sql.ErrNoRows {
		return Group{}, ErrNoSuchGroup
	}
	return g, err
}

func (s *Store) GroupByID(id int64) (Group, error) {
	return scanGroup(s.db.QueryRow(groupSelect+` WHERE g.project_id = ? AND g.id = ?`, s.projectID, id))
}

// The live group the tab belongs to as pilot or member; ErrNoSuchGroup when it has none.
func (s *Store) LiveGroupForTab(tabID string) (Group, error) {
	return scanGroup(s.db.QueryRow(groupSelect+`
		JOIN lg_group_members m ON m.group_id = g.id
		WHERE g.project_id = ? AND g.disbanded_at IS NULL AND m.tab_id = ?
	`, s.projectID, tabID))
}

func (s *Store) GroupMembers(groupID int64) ([]Member, error) {
	rows, err := s.db.Query(`SELECT tab_id, role, label, vendor, prompt, skills FROM lg_group_members WHERE group_id = ? ORDER BY CASE role WHEN 'pilot' THEN 0 ELSE 1 END, rowid`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		var skills string
		if err := rows.Scan(&m.TabID, &m.Role, &m.Label, &m.Vendor, &m.Prompt, &skills); err != nil {
			return nil, err
		}
		if skills != "" {
			m.Skills = strings.Split(skills, ",")
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// The tab's live group and its own member row; ErrNoSuchGroup when the tab is in no live group.
func (s *Store) LiveMembership(tabID string) (Group, Member, error) {
	group, err := s.LiveGroupForTab(tabID)
	if err != nil {
		return Group{}, Member{}, err
	}
	members, err := s.GroupMembers(group.ID)
	if err != nil {
		return Group{}, Member{}, err
	}
	for _, m := range members {
		if m.TabID == tabID {
			return group, m, nil
		}
	}
	return Group{}, Member{}, ErrNoSuchGroup
}

// Keeps the thread and its messages, so a disbanded group stays readable.
func (s *Store) DisbandGroup(id int64) error {
	res, err := s.db.Exec(`UPDATE lg_groups SET disbanded_at = ? WHERE project_id = ? AND id = ? AND disbanded_at IS NULL`, now(), s.projectID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GroupByID(id); err != nil {
			return err
		}
		return fmt.Errorf("session: workgroup %d is already disbanded", id)
	}
	return nil
}

// Removes a group that never got going, with its members and its empty thread.
func (s *Store) DeleteGroup(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM lg_threads WHERE group_id = ? AND project_id = ? AND NOT EXISTS (SELECT 1 FROM lg_messages WHERE thread_id = lg_threads.id)`, id, s.projectID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM lg_group_members WHERE group_id = ? AND group_id IN (SELECT id FROM lg_groups WHERE project_id = ?)`, id, s.projectID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM lg_groups WHERE id = ? AND project_id = ?`, id, s.projectID); err != nil {
		return err
	}
	return tx.Commit()
}

// Maps every requested tab id to its group label, taken from its most recent membership in this project, or to its short id when it has none.
func (s *Store) Labels(tabIDs []string) map[string]string {
	out := make(map[string]string, len(tabIDs))
	if len(tabIDs) == 0 {
		return out
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(tabIDs)), ",")
	args := []interface{}{s.projectID}
	for _, id := range tabIDs {
		out[id] = TabLabel(id)
		args = append(args, id)
	}
	rows, err := s.db.Query(`
		SELECT m.tab_id, m.label FROM lg_group_members m JOIN lg_groups g ON g.id = m.group_id
		WHERE g.project_id = ? AND m.tab_id IN (`+marks+`) ORDER BY g.id
	`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, label string
		if rows.Scan(&id, &label) == nil {
			out[id] = label
		}
	}
	return out
}

// The copilot skill first, then the skills the pilot listed, each once.
func (m Member) RequiredSkills() []string {
	out := []string{CopilotSkill}
	for _, name := range m.Skills {
		if !containsString(out, name) {
			out = append(out, name)
		}
	}
	return out
}

func (m Member) Requires(skill string) bool {
	return containsString(m.RequiredSkills(), skill)
}
