package wa

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lugvitc/whats4linux/internal/db"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type ContainerWrapper struct {
	db *sql.DB
	c  *sqlstore.Container
}

func NewContainerWrapper(ctx context.Context, dialect, address string, log waLog.Logger) (*ContainerWrapper, error) {
	db, err := sql.Open(dialect, address)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	container := sqlstore.NewWithDB(db, dialect, log)
	err = container.Upgrade(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade database: %w", err)
	}
	return &ContainerWrapper{
		db: db,
		c:  container,
	}, nil
}

func (cw *ContainerWrapper) GetContainer() *sqlstore.Container {
	return cw.c
}

func (cw *ContainerWrapper) Initialise(ctx context.Context, client *whatsmeow.Client) error {
	_, err := cw.db.Exec(query.CreateGroupsTable)
	if err != nil {
		return fmt.Errorf("failed to create whats4linux_groups table: %w", err)
	}

	err = cw.FetchAndStoreGroups(context.Background(), client)
	if err != nil {
		return fmt.Errorf("failed to fetch and store groups: %w", err)
	}
	return nil
}

func (cw *ContainerWrapper) FetchAndStoreGroups(ctx context.Context, client *whatsmeow.Client) error {
	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch joined groups: %w", err)
	}

	tx, err := cw.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(query.InsertOrReplaceGroup)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, group := range groups {
		_, err := stmt.Exec(
			group.JID.String(),
			group.Name,
			group.Topic,
			group.OwnerJID.String(),
			len(group.Participants),
		)
		if err != nil {
			return fmt.Errorf("failed to insert group %s: %w", group.JID.String(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

type Group struct {
	JID              string
	Name             string
	Topic            string
	OwnerJID         string
	ParticipantCount int
}

func (cw *ContainerWrapper) FetchGroups() ([]Group, error) {
	rows, err := cw.db.Query(query.SelectAllGroups)
	if err != nil {
		return nil, fmt.Errorf("failed to query groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		err := rows.Scan(&g.JID, &g.Name, &g.Topic, &g.OwnerJID, &g.ParticipantCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan group row: %w", err)
		}
		groups = append(groups, g)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return groups, nil
}

func (cw *ContainerWrapper) FetchGroup(jid string) (*Group, error) {
	row := cw.db.QueryRow(query.SelectGroupByJID, jid)

	var g Group
	err := row.Scan(&g.JID, &g.Name, &g.Topic, &g.OwnerJID, &g.ParticipantCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("group with JID %s not found", jid)
		}
		return nil, fmt.Errorf("failed to scan group row: %w", err)
	}

	return &g, nil
}

func (cw *ContainerWrapper) Close() error {
	return cw.db.Close()
}
