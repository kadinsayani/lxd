package cluster

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
)

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t placement_groups.mapper.go
//go:generate mapper reset -i -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e placement_group objects table=placement_groups
//go:generate mapper stmt -e placement_group objects-by-ID table=placement_groups
//go:generate mapper stmt -e placement_group objects-by-Project table=placement_groups
//go:generate mapper stmt -e placement_group objects-by-Name-and-Project table=placement_groups
//go:generate mapper stmt -e placement_group id table=placement_groups
//go:generate mapper stmt -e placement_group create struct=PlacementGroup table=placement_groups
//go:generate mapper stmt -e placement_group delete-by-Name-and-Project table=placement_groups
//go:generate mapper stmt -e placement_group update struct=PlacementGroup table=placement_groups
//go:generate mapper stmt -e placement_group rename struct=PlacementGroup table=placement_groups
//
//go:generate mapper method -i -e placement_group GetMany
//go:generate mapper method -i -e placement_group GetOne
//go:generate mapper method -i -e placement_group ID struct=PlacementGroup
//go:generate mapper method -i -e placement_group Exists struct=PlacementGroup
//go:generate mapper method -i -e placement_group Create struct=PlacementGroup
//go:generate mapper method -i -e placement_group DeleteOne-by-Name-and-Project
//go:generate mapper method -i -e placement_group Update struct=PlacementGroup
//go:generate mapper method -i -e placement_group Rename struct=PlacementGroup
//go:generate goimports -w placement_groups.mapper.go
//go:generate goimports -w placement_groups.interface.mapper.go

// PlacementGroup is the database representation of an [api.PlacementGroup].
type PlacementGroup struct {
	ID          int
	Name        string `db:"primary=yes"`
	Project     string `db:"primary=yes&join=projects.name"`
	Description string `db:"coalesce=''"`
}

// PlacementGroupFilter contains fields that can be used to filter results when getting placement groups.
type PlacementGroupFilter struct {
	ID      *int
	Project *string
	Name    *string
}

// CreatePlacementGroupConfig creates config for a new placement group with the given ID.
func CreatePlacementGroupConfig(ctx context.Context, tx *sql.Tx, placementGroupID int64, config map[string]string) error {
	stmt, err := tx.Prepare("INSERT INTO placement_groups_config (placement_group_id, key, value) VALUES(?, ?, ?)")
	if err != nil {
		return err
	}

	defer func() { _ = stmt.Close() }()

	for k, v := range config {
		if v == "" {
			continue
		}

		_, err = stmt.Exec(placementGroupID, k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdatePlacementGroupConfig updates the placement group config with the given ID.
func UpdatePlacementGroupConfig(ctx context.Context, tx *sql.Tx, placementGroupID int64, config map[string]string) error {
	// Delete current entries.
	_, err := tx.Exec("DELETE FROM placement_groups_config WHERE placement_group_id=?", placementGroupID)
	if err != nil {
		return err
	}

	// Insert new entries.
	return CreatePlacementGroupConfig(ctx, tx, placementGroupID, config)
}

// GetPlacementGroupConfig returns the config for the placement group with the given ID.
func GetPlacementGroupConfig(ctx context.Context, tx *sql.Tx, placementGroupID int) (map[string]string, error) {
	q := `SELECT key, value FROM placement_groups_config WHERE placement_group_id=?`

	config := map[string]string{}
	return config, query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var key, value string

		err := scan(&key, &value)
		if err != nil {
			return err
		}

		_, found := config[key]
		if found {
			return fmt.Errorf("Duplicate config row found for key %q for placement group ID %d", key, placementGroupID)
		}

		config[key] = value
		return nil
	}, placementGroupID)
}

// ToAPIBase populates base fields of the [PlacementGroup] into an [api.PlacementGroup] without querying for any additional data.
// This is so that additional fields can be populated elsewhere when performing bulk queries.
func (p PlacementGroup) ToAPIBase() api.PlacementGroup {
	return api.PlacementGroup{
		Name:        p.Name,
		Description: p.Description,
		Project:     p.Project,
	}
}

// ToAPI converts the [PlacementGroup] to an [api.PlacementGroup], querying for extra data as necessary.
func (p *PlacementGroup) ToAPI(ctx context.Context, tx *sql.Tx) (*api.PlacementGroup, error) {
	// Get config
	config, err := GetPlacementGroupConfig(ctx, tx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("Failed getting placement group config: %w", err)
	}

	// Get used by
	usedBy, err := GetPlacementGroupUsedBy(ctx, tx, p.Project, p.Name)
	if err != nil {
		return nil, err
	}

	apiPlacementGroup := p.ToAPIBase()
	apiPlacementGroup.UsedBy = usedBy
	apiPlacementGroup.Config = config

	return &apiPlacementGroup, nil
}

// GetPlacementGroupUsedBy returns a list of URLs of all instances and profiles that reference the given placement group in their configuration.
func GetPlacementGroupUsedBy(ctx context.Context, tx *sql.Tx, projectName string, name string) ([]string, error) {
	q := `SELECT ` + strconv.Itoa(int(entityTypeCodeInstance)) + `, instances.name FROM instances
JOIN instances_config ON instances.id = instances_config.instance_id
JOIN projects ON instances.project_id = projects.id
WHERE instances_config.key = 'placement.group' AND instances_config.value = ? AND projects.name = ?
UNION SELECT ` + strconv.Itoa(int(entityTypeCodeProfile)) + `, profiles.name FROM profiles
JOIN profiles_config ON profiles.id = profiles_config.profile_id
JOIN projects ON profiles.project_id = projects.id
WHERE profiles_config.key = 'placement.group' AND profiles_config.value = ? AND projects.name = ?
`

	var urls []string
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var eType EntityType
		var eName string
		err := scan(&eType, &eName)
		if err != nil {
			return err
		}

		switch entity.Type(eType) {
		case entity.TypeInstance:
			urls = append(urls, api.NewURL().Project(projectName).Path("1.0", "instances", eName).String())
		case entity.TypeProfile:
			urls = append(urls, api.NewURL().Project(projectName).Path("1.0", "profiles", eName).String())
		}

		return nil
	}, name, projectName, name, projectName)
	if err != nil {
		return nil, fmt.Errorf("Failed finding references to placement group %q: %w", name, err)
	}

	return urls, nil
}

// GetAllPlacementGroupUsedByURLs returns a map of project name to map of placement group name to a list of URLs of instances and
// profiles that reference the placement group in their configuration. If a project is given, used by URLs will only be returned for placement groups in that project.
func GetAllPlacementGroupUsedByURLs(ctx context.Context, tx *sql.Tx, project *string) (map[string]map[string][]string, error) {
	var b strings.Builder
	var args []any
	b.WriteString(`SELECT ` + strconv.Itoa(int(entityTypeCodeInstance)) + `, instances.name, projects.name, instances_config.value FROM instances
JOIN instances_config ON instances.id = instances_config.instance_id
JOIN projects ON projects.id = instances.project_id
WHERE instances_config.key = 'placement.group'`)
	if project != nil {
		b.WriteString(" AND projects.name = ?\n")
		args = append(args, *project)
	}

	b.WriteString(`UNION SELECT ` + strconv.Itoa(int(entityTypeCodeProfile)) + `, profiles.name, projects.name, profiles_config.value FROM profiles
	JOIN profiles_config ON profiles.id = profiles_config.profile_id
	JOIN projects ON projects.id = profiles.project_id
	WHERE profiles_config.key = 'placement.group'`)
	if project != nil {
		b.WriteString(" AND projects.name = ?")
		args = append(args, *project)
	}

	urlMap := make(map[string]map[string][]string)
	err := query.Scan(ctx, tx, b.String(), func(scan func(dest ...any) error) error {
		var eType EntityType
		var eName string
		var projectName string
		var placementGroupName string
		err := scan(&eType, &eName, &projectName, &placementGroupName)
		if err != nil {
			return err
		}

		var u string
		switch entity.Type(eType) {
		case entity.TypeInstance:
			u = api.NewURL().Project(projectName).Path("1.0", "instances", eName).String()
		case entity.TypeProfile:
			u = api.NewURL().Project(projectName).Path("1.0", "profiles", eName).String()
		default:
			return errors.New("Unexpected entity type in placement group usage query")
		}

		projectMap, ok := urlMap[projectName]
		if !ok {
			urlMap[projectName] = map[string][]string{
				placementGroupName: {u},
			}

			return nil
		}

		projectMap[placementGroupName] = append(projectMap[placementGroupName], u)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed retrieving used by URLs for placement groups: %w", err)
	}

	return urlMap, nil
}

// GetPlacementGroupNames returns a map of project name to slice of placement group names. If a project name is provided,
// only groups in that project are returned. Otherwise, the returned map will contain all projects.
func GetPlacementGroupNames(ctx context.Context, tx *sql.Tx, project *string) (map[string][]string, error) {
	var b strings.Builder
	b.WriteString(`
SELECT
	projects.name,
	placement_groups.name
FROM placement_groups
JOIN projects ON projects.id = placement_groups.project_id
`)

	var args []any
	if project != nil {
		b.WriteString(`WHERE projects.name = ?`)
		args = []any{*project}
	}

	nameMap := make(map[string][]string)
	err := query.Scan(ctx, tx, b.String(), func(scan func(dest ...any) error) error {
		var projectName string
		var placementGroupName string
		err := scan(&projectName, &placementGroupName)
		if err != nil {
			return err
		}

		nameMap[projectName] = append(nameMap[projectName], placementGroupName)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed querying placement group names: %w", err)
	}

	return nameMap, nil
}
