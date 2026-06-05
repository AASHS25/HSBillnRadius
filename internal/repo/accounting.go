package repo

import (
	"context"
	"fmt"

	"github.com/aashs25/hsbillnradius/internal/domain/radius"
	"github.com/aashs25/hsbillnradius/internal/repo/sqlc"
)

type accountingRepo struct{ q *sqlc.Queries }

func (r *accountingRepo) Start(ctx context.Context, e radius.AcctEvent) error {
	if err := r.q.InsertAcctStart(ctx, sqlc.InsertAcctStartParams{
		TenantID:         e.TenantID,
		Acctsessionid:    e.SessionID,
		Acctuniqueid:     e.UniqueID,
		Username:         e.Username,
		Nasipaddress:     e.NASIP,
		Nasportid:        e.NASPort,
		Acctstarttime:    e.StartTime,
		Framedipaddress:  e.FramedIP,
		Callingstationid: e.CallingStation,
		Calledstationid:  e.CalledStation,
	}); err != nil {
		return fmt.Errorf("acct start: %w", err)
	}
	return nil
}

func (r *accountingRepo) Interim(ctx context.Context, e radius.AcctEvent) error {
	if err := r.q.UpdateAcctInterim(ctx, sqlc.UpdateAcctInterimParams{
		Acctuniqueid:     e.UniqueID,
		Acctsessiontime:  e.SessionTime,
		Acctinputoctets:  e.InputOctets,
		Acctoutputoctets: e.OutputOctets,
		Column5:          e.FramedIP,
	}); err != nil {
		return fmt.Errorf("acct interim: %w", err)
	}
	return nil
}

func (r *accountingRepo) Stop(ctx context.Context, e radius.AcctEvent) error {
	if err := r.q.UpdateAcctStop(ctx, sqlc.UpdateAcctStopParams{
		Acctuniqueid:       e.UniqueID,
		Acctsessiontime:    e.SessionTime,
		Acctinputoctets:    e.InputOctets,
		Acctoutputoctets:   e.OutputOctets,
		Acctterminatecause: e.TerminateCause,
	}); err != nil {
		return fmt.Errorf("acct stop: %w", err)
	}
	return nil
}

func (r *accountingRepo) ActiveSessions(ctx context.Context, tenantID int64, username string) ([]radius.ActiveSession, error) {
	rows, err := r.q.ListActiveSessions(ctx, sqlc.ListActiveSessionsParams{TenantID: tenantID, Username: username})
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	out := make([]radius.ActiveSession, len(rows))
	for i, m := range rows {
		out[i] = radius.ActiveSession{
			SessionID:      m.Acctsessionid,
			NASIP:          m.Nasipaddress,
			FramedIP:       m.Framedipaddress,
			CallingStation: m.Callingstationid,
		}
	}
	return out, nil
}
