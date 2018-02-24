package helpers

import (
	"database/sql"

	"github.com/talkative-ai/core/db"
)

func CreateVersionedProject(tx *sql.Tx, projectID string, version int64) error {
	submitQuery := `
		INSERT INTO static_published_projects_versioned
			("ProjectID", "Version", "Title", "Category", "Tags", "ProjectData", "TriggerData")
		SELECT
			$1 "ProjectID",
			$2 "Version",
			p."Title",
			p."Category",
			p."Tags",
			COALESCE((
				SELECT jsonb_agg(data)
				FROM (
					SELECT DISTINCT
						za."ActorID",
						za."ZoneID",

						d."ID" "DialogID",
						d."EntryInput" "DialogEntry",
						d."AlwaysExec",
						d."Statements",
						d."IsRoot",
						d."UnknownHandler",
						
						dr."ParentNodeID" "ParentDialogID",
						dr."ChildNodeID" "ChildDialogID"

						FROM workbench_projects p
						JOIN workbench_zones z
							ON z."ProjectID" = p."ID"
						JOIN workbench_zones_actors za
							ON za."ZoneID"=z."ID"
						JOIN workbench_dialog_nodes d
							ON d."ActorID"=za."ActorID"
						FULL OUTER JOIN workbench_dialog_nodes_relations dr
							ON dr."ParentNodeID"=d."ID" OR dr."ChildNodeID"=d."ID"
						WHERE p."ID"=$1
				) data
			), '[]'::jsonb) AS "ProjectData",
			COALESCE((
				SELECT jsonb_agg(triggers)
				FROM (
					SELECT DISTINCT
						zone."ID" "ZoneID",
						trig."TriggerType",
						trig."AlwaysExec",
						trig."Statements"

					FROM workbench_zones zone
					JOIN workbench_triggers trig
						ON trig."ZoneID"=zone."ID"
					WHERE zone."ProjectID"=$1
				) triggers
			), '[]'::jsonb) AS "TriggerData"
		FROM (
			SELECT
				"Title",
				"Category",
				"Tags"
				FROM workbench_projects
				WHERE "ID"=$1 LIMIT 1) p
		GROUP BY (p."Title", p."Category", p."Tags")
	`

	txWasNil := false

	if tx == nil {
		var err error
		txWasNil = true
		tx, err = db.Instance.Begin()
		if err != nil {
			return err
		}
	}

	tx.Exec(`
		DELETE FROM static_published_projects_versioned
		WHERE "ProjectID"=$1
		AND "Version"=$2
		`, projectID, version)

	tx.Exec(submitQuery, projectID, version)

	if txWasNil == true {
		return tx.Commit()
	}

	return nil
}
