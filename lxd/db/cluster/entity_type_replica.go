package cluster

import (
	"fmt"
)

// entityTypeReplica implements entityTypeDBInfo for a [Replica].
type entityTypeReplica struct{}

func (e entityTypeReplica) code() int64 {
	return entityTypeCodeReplica
}

func (e entityTypeReplica) allURLsQuery() string {
	return fmt.Sprintf(`SELECT %d, replicas.id, '', '', json_array(replicas.name) FROM replicas`, e.code())
}

func (e entityTypeReplica) urlsByProjectQuery() string {
	return ""
}

func (e entityTypeReplica) urlByIDQuery() string {
	return e.allURLsQuery() + " WHERE replicas.id = ?"
}

func (e entityTypeReplica) idFromURLQuery() string {
	return `
SELECT ?, replicas.id 
FROM replicas 
WHERE '' = ? 
	AND '' = ? 
	AND replicas.name = ?`
}

func (e entityTypeReplica) onDeleteTriggerSQL() (name string, sql string) {
	name = "on_replica_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON replicas
	BEGIN
	DELETE FROM auth_groups_permissions 
		WHERE entity_type = %d 
		AND entity_id = OLD.id;
	DELETE FROM warnings 
		WHERE entity_type_code = %d 
		AND entity_id = OLD.id;
	END
`, name, e.code(), e.code())
}
