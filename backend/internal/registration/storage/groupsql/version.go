package groupsql

import "pagasacentre/backend/internal/registration/domain"

func CheckVersion(g *domain.Group, meta domain.ActionMeta) error {
	if !meta.EnforceVersion() {
		return nil
	}
	if g == nil || g.Version != meta.ExpectedVersion {
		return domain.ErrVersionConflict
	}
	return nil
}
