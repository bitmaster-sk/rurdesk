package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/lexorank"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikimerge"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikitext"
	"github.com/jackc/pgx/v5"
)

const (
	wikiEditorTTL     = 30 * time.Second
	wikiEditorKeyBase = "wiki:editing:"
)

type WikiSaveResult struct {
	Page       *model.WikiPage   `json:"page"`
	MergedFrom *int              `json:"mergedFrom"`
	Conflict   *WikiConflictInfo `json:"conflict,omitempty"`
}

type WikiConflictInfo struct {
	Current     *model.WikiPage           `json:"current"`
	Title       string                    `json:"title"`
	Summary     string                    `json:"summary"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess"`
	Merge       wikimerge.Result          `json:"merge"`
}

type WikiMergePreview struct {
	CurrentVersion int                       `json:"currentVersion"`
	Title          string                    `json:"title"`
	Summary        string                    `json:"summary"`
	AgentAccess    constants.WikiAgentAccess `json:"agentAccess"`
	Merge          wikimerge.Result          `json:"merge"`
}

type wikiMergedFields struct {
	title       string
	summary     string
	agentAccess constants.WikiAgentAccess
}

type wikiEditorEntry struct {
	Name   string `json:"name"`
	SeenAt int64  `json:"seenAt"`
}

func (s *WikiService) Save(ctx context.Context, user model.User, idPage int64, req model.SaveWikiPageReq) (*WikiSaveResult, error) {
	if wikitext.Slugify(req.Title) == "" {
		return nil, errs.ErrWikiInvalidTitle
	}
	var result *WikiSaveResult
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		page, access, err := s.loadPageAccess(ctx, user, idPage, true)
		if err != nil {
			return err
		}
		if page.DeletedAt != nil {
			return errs.ErrNotFound
		}
		if !access.canEdit {
			return errs.ErrForbidden
		}
		if req.BaseVersion > page.VersionNo {
			return errs.ErrBadRequest
		}

		title, summary, body, agentAccess := req.Title, req.Summary, req.Body, req.AgentAccess
		var mergedFrom *int
		if req.BaseVersion < page.VersionNo {
			base, err := s.versionRepo.LoadOne(ctx, page.IdPage, req.BaseVersion)
			if err != nil {
				return err
			}
			if base == nil {
				return errs.ErrBadRequest
			}
			merged := wikimerge.Merge(base.Body, req.Body, page.Body)
			fields := mergeWikiFields(base, req.Title, req.Summary, req.AgentAccess, page)
			if merged.HasConflicts() {
				result = &WikiSaveResult{Conflict: &WikiConflictInfo{
					Current:     page,
					Title:       fields.title,
					Summary:     fields.summary,
					AgentAccess: fields.agentAccess,
					Merge:       merged,
				}}
				return nil
			}
			body = merged.Text()
			title, summary, agentAccess = fields.title, fields.summary, fields.agentAccess
			currentVersion := page.VersionNo
			mergedFrom = &currentVersion
		}

		touchesAlways := (agentAccess == constants.WikiAgentAccessAlways) != (page.AgentAccess == constants.WikiAgentAccessAlways)
		if touchesAlways && !access.canManage {
			return errs.ErrForbidden
		}
		if title == page.Title && summary == page.Summary && body == page.Body && agentAccess == page.AgentAccess {
			if err := s.draftRepo.Delete(ctx, page.IdPage, user.IdUser); err != nil {
				return err
			}
			result = &WikiSaveResult{Page: page}
			return nil
		}
		if agentAccess == constants.WikiAgentAccessAlways {
			if err := s.checkBudget(ctx, access.space, []int64{page.IdPage}, wikiChars(title, body), alwaysCharsOf(page)); err != nil {
				return err
			}
		}

		page.Title, page.Summary, page.Body, page.AgentAccess = title, summary, body, agentAccess
		page.VersionNo++
		if err := s.writeVersion(ctx, page, access.space, req.Note, mergedFrom, user); err != nil {
			return err
		}
		result = &WikiSaveResult{Page: page, MergedFrom: mergedFrom}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *WikiService) MergePreview(ctx context.Context, user model.User, idPage int64, req model.WikiMergePreviewReq) (*WikiMergePreview, error) {
	page, _, err := s.loadPageAccess(ctx, user, idPage, false)
	if err != nil {
		return nil, err
	}
	base, err := s.versionRepo.LoadOne(ctx, page.IdPage, req.BaseVersion)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, errs.ErrBadRequest
	}
	fields := mergeWikiFields(base, req.Title, req.Summary, req.AgentAccess, page)
	return &WikiMergePreview{
		CurrentVersion: page.VersionNo,
		Title:          fields.title,
		Summary:        fields.summary,
		AgentAccess:    fields.agentAccess,
		Merge:          wikimerge.Merge(base.Body, req.Body, page.Body),
	}, nil
}

func (s *WikiService) Revert(ctx context.Context, user model.User, idPage int64, versionNo int) (*model.WikiPage, error) {
	var reverted *model.WikiPage
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		page, access, err := s.loadPageAccess(ctx, user, idPage, true)
		if err != nil {
			return err
		}
		if page.DeletedAt != nil {
			return errs.ErrNotFound
		}
		if !access.canEdit {
			return errs.ErrForbidden
		}
		old, err := s.versionRepo.LoadOne(ctx, page.IdPage, versionNo)
		if err != nil {
			return err
		}
		if old == nil {
			return errs.ErrNotFound
		}
		touchesAlways := (old.AgentAccess == constants.WikiAgentAccessAlways) != (page.AgentAccess == constants.WikiAgentAccessAlways)
		if touchesAlways && !access.canManage {
			return errs.ErrForbidden
		}
		if old.AgentAccess == constants.WikiAgentAccessAlways {
			if err := s.checkBudget(ctx, access.space, []int64{page.IdPage}, wikiChars(old.Title, old.Body), alwaysCharsOf(page)); err != nil {
				return err
			}
		}
		page.Title, page.Summary, page.Body, page.AgentAccess = old.Title, old.Summary, old.Body, old.AgentAccess
		page.VersionNo++
		note := "restored v" + strconv.Itoa(versionNo)
		if err := s.writeVersion(ctx, page, access.space, note, nil, user); err != nil {
			return err
		}
		reverted = page
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reverted, nil
}

func (s *WikiService) LoadVersions(ctx context.Context, user model.User, idPage int64) ([]*model.WikiPageVersionSummary, error) {
	if _, _, err := s.loadPageAccess(ctx, user, idPage, false); err != nil {
		return nil, err
	}
	return s.versionRepo.Load(ctx, idPage)
}

func (s *WikiService) LoadVersion(ctx context.Context, user model.User, idPage int64, versionNo int) (*model.WikiPageVersion, error) {
	if _, _, err := s.loadPageAccess(ctx, user, idPage, false); err != nil {
		return nil, err
	}
	version, err := s.versionRepo.LoadOne(ctx, idPage, versionNo)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, errs.ErrNotFound
	}
	return version, nil
}

func (s *WikiService) Diff(ctx context.Context, user model.User, idPage int64, fromVersion, toVersion int) (string, error) {
	from, err := s.LoadVersion(ctx, user, idPage, fromVersion)
	if err != nil {
		return "", err
	}
	to, err := s.LoadVersion(ctx, user, idPage, toVersion)
	if err != nil {
		return "", err
	}
	return wikimerge.UnifiedDiff(withTrailingNewline(from.Body), withTrailingNewline(to.Body), to.Title, to.Title)
}

func (s *WikiService) SaveDraft(ctx context.Context, user model.User, idPage int64, req model.WikiDraftReq) error {
	page, access, err := s.loadPageAccess(ctx, user, idPage, false)
	if err != nil {
		return err
	}
	if !access.canEdit {
		return errs.ErrForbidden
	}
	return s.draftRepo.Upsert(ctx, &model.WikiPageDraft{
		IdPage: page.IdPage, IdUser: user.IdUser, BaseVersion: req.BaseVersion,
		Title: req.Title, Summary: req.Summary, Body: req.Body,
	})
}

func (s *WikiService) LoadDraft(ctx context.Context, user model.User, idPage int64) (*model.WikiPageDraft, error) {
	if _, _, err := s.loadPageAccess(ctx, user, idPage, false); err != nil {
		return nil, err
	}
	return s.draftRepo.Load(ctx, idPage, user.IdUser)
}

func (s *WikiService) DeleteDraft(ctx context.Context, user model.User, idPage int64) error {
	if _, _, err := s.loadPageAccess(ctx, user, idPage, false); err != nil {
		return err
	}
	return s.draftRepo.Delete(ctx, idPage, user.IdUser)
}

func (s *WikiService) Move(ctx context.Context, user model.User, idPage int64, req model.MoveWikiPageReq) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		page, access, err := s.lockStructure(ctx, user, idPage)
		if err != nil {
			return err
		}
		if page.DeletedAt != nil {
			return errs.ErrNotFound
		}
		if !access.canEdit {
			return errs.ErrForbidden
		}
		if err := s.validateParent(ctx, page.IdSpace, req.IdParent, page.IdPage); err != nil {
			return err
		}
		prev, err := s.siblingRank(ctx, page.IdSpace, req.IdParent, req.IdPrev)
		if err != nil {
			return err
		}
		next, err := s.siblingRank(ctx, page.IdSpace, req.IdParent, req.IdNext)
		if err != nil {
			return err
		}
		if req.IdPrev == nil && req.IdNext == nil {
			last, err := s.pageRepo.LoadLastSiblingRank(ctx, page.IdSpace, req.IdParent)
			if err != nil {
				return err
			}
			if last != nil {
				prev = *last
			}
		}
		if prev != "" && next != "" && prev >= next {
			return errs.ErrBadRequest
		}
		return s.pageRepo.UpdatePosition(ctx, page.IdPage, req.IdParent, lexorank.Between(prev, next))
	})
}

func (s *WikiService) Trash(ctx context.Context, user model.User, idPage int64) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		page, access, err := s.lockStructure(ctx, user, idPage)
		if err != nil {
			return err
		}
		if page.DeletedAt != nil {
			return errs.ErrNotFound
		}
		if !access.canEdit {
			return errs.ErrForbidden
		}
		ids, err := s.pageRepo.LoadLiveSubtreeIds(ctx, page.IdPage)
		if err != nil {
			return err
		}
		return s.pageRepo.Trash(ctx, page.IdPage, ids, user.IdUser)
	})
}

func (s *WikiService) LoadTrash(ctx context.Context, user model.User, idProject int64) ([]*model.WikiTrashItem, error) {
	instance, project, err := s.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	return s.pageRepo.LoadTrash(ctx, s.trashSpaceIds(ctx, user, instance, project), user.IsAgent)
}

func (s *WikiService) Restore(ctx context.Context, user model.User, idPage int64) (*model.WikiPage, error) {
	var restored *model.WikiPage
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		page, access, err := s.lockStructure(ctx, user, idPage)
		if err != nil {
			return err
		}
		if page.IdDeletedRoot == nil || *page.IdDeletedRoot != page.IdPage {
			return errs.ErrNotFound
		}
		if !access.canEdit {
			return errs.ErrForbidden
		}
		trashed, err := s.pageRepo.LoadTrashedPages(ctx, page.IdPage)
		if err != nil {
			return err
		}
		alwaysChars, hasAlways := 0, false
		for _, trashedPage := range trashed {
			if trashedPage.AgentAccess == constants.WikiAgentAccessAlways {
				hasAlways = true
				alwaysChars += wikiChars(trashedPage.Title, trashedPage.Body)
			}
		}
		if hasAlways {
			if !access.canManage {
				return errs.ErrForbidden
			}
			if err := s.checkBudget(ctx, access.space, nil, alwaysChars, 0); err != nil {
				return err
			}
		}
		for _, trashedPage := range trashed {
			taken, err := s.pageRepo.IsSlugTaken(ctx, trashedPage.IdSpace, trashedPage.Slug)
			if err != nil {
				return err
			}
			if taken {
				return errs.ErrWikiSlugTaken.WithMessage("a live page already uses the slug " + trashedPage.Slug)
			}
		}
		if err := s.pageRepo.Restore(ctx, page.IdPage); err != nil {
			return err
		}
		if page.IdParent != nil {
			parent, err := s.pageRepo.LoadPage(ctx, *page.IdParent)
			if err != nil {
				return err
			}
			if parent == nil || parent.DeletedAt != nil {
				last, err := s.pageRepo.LoadLastSiblingRank(ctx, page.IdSpace, nil)
				if err != nil {
					return err
				}
				prev := ""
				if last != nil {
					prev = *last
				}
				page.IdParent = nil
				page.Rank = lexorank.Between(prev, "")
				if err := s.pageRepo.UpdatePosition(ctx, page.IdPage, nil, page.Rank); err != nil {
					return err
				}
			}
		}
		page.DeletedAt, page.DeletedBy, page.IdDeletedRoot = nil, nil, nil
		restored = page
		return nil
	})
	if err != nil {
		return nil, err
	}
	return restored, nil
}

func (s *WikiService) Purge(ctx context.Context, user model.User, idPage int64) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		page, access, err := s.lockStructure(ctx, user, idPage)
		if err != nil {
			return err
		}
		if page.IdDeletedRoot == nil || *page.IdDeletedRoot != page.IdPage {
			return errs.ErrNotFound
		}
		if !access.canManage {
			return errs.ErrForbidden
		}
		return s.pageRepo.Purge(ctx, page.IdPage)
	})
}

func (s *WikiService) PurgeExpiredTrash(ctx context.Context, now time.Time) (int64, error) {
	var purged int64
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if err := s.spaceRepo.LockAllSpaces(ctx); err != nil {
			return err
		}
		count, err := s.pageRepo.PurgeTrashedBefore(ctx, now.AddDate(0, 0, -constants.WikiTrashRetentionDays))
		purged = count
		return err
	})
	return purged, err
}

func (s *WikiService) Heartbeat(ctx context.Context, user model.User, idPage int64, isEditing bool) ([]model.WikiEditor, error) {
	if _, _, err := s.loadPageAccess(ctx, user, idPage, false); err != nil {
		return nil, err
	}
	key := wikiEditorKeyBase + strconv.FormatInt(idPage, 10)
	now := time.Now()
	field := strconv.FormatInt(user.IdUser, 10)
	if isEditing {
		value, err := json.Marshal(wikiEditorEntry{Name: user.Name, SeenAt: now.Unix()})
		if err != nil {
			return nil, err
		}
		if err := s.cache.HSet(ctx, key, field, value).Err(); err != nil {
			return nil, fmt.Errorf("storing wiki editor: %w", err)
		}
		if err := s.cache.Expire(ctx, key, wikiEditorTTL*2).Err(); err != nil {
			return nil, fmt.Errorf("expiring wiki editors: %w", err)
		}
	} else if err := s.cache.HDel(ctx, key, field).Err(); err != nil {
		return nil, fmt.Errorf("removing wiki editor: %w", err)
	}
	entries, err := s.cache.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("loading wiki editors: %w", err)
	}
	editors := make([]model.WikiEditor, 0, len(entries))
	for idText, raw := range entries {
		var entry wikiEditorEntry
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			continue
		}
		idEditor, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || idEditor == user.IdUser {
			continue
		}
		if now.Sub(time.Unix(entry.SeenAt, 0)) > wikiEditorTTL {
			continue
		}
		editors = append(editors, model.WikiEditor{IdUser: idEditor, Name: entry.Name})
	}
	return editors, nil
}

func mergeWikiFields(base *model.WikiPageVersion, title, summary string, agentAccess constants.WikiAgentAccess, current *model.WikiPage) wikiMergedFields {
	if agentAccess == "" {
		agentAccess = base.AgentAccess
	}
	return wikiMergedFields{
		title:       wikimerge.MergeField(base.Title, title, current.Title),
		summary:     wikimerge.MergeField(base.Summary, summary, current.Summary),
		agentAccess: constants.WikiAgentAccess(wikimerge.MergeField(string(base.AgentAccess), string(agentAccess), string(current.AgentAccess))),
	}
}

func alwaysCharsOf(page *model.WikiPage) int {
	if page.AgentAccess != constants.WikiAgentAccessAlways {
		return 0
	}
	return wikiChars(page.Title, page.Body)
}

func (s *WikiService) lockStructure(ctx context.Context, user model.User, idPage int64) (*model.WikiPage, wikiAccess, error) {
	page, _, err := s.loadPageAccess(ctx, user, idPage, false)
	if err != nil {
		return nil, wikiAccess{}, err
	}
	if err := s.spaceRepo.LockSpace(ctx, page.IdSpace); err != nil {
		return nil, wikiAccess{}, err
	}
	return s.loadPageAccess(ctx, user, idPage, true)
}

func (s *WikiService) writeVersion(ctx context.Context, page *model.WikiPage, space *model.WikiSpace, note string, mergedFrom *int, user model.User) error {
	if err := s.pageRepo.UpdateContent(ctx, page, user.IdUser); err != nil {
		return err
	}
	if err := s.versionRepo.Insert(ctx, versionOf(page, note, mergedFrom, user.IdUser)); err != nil {
		return err
	}
	if err := s.storeLinks(ctx, page, space); err != nil {
		return err
	}
	if err := s.draftRepo.Delete(ctx, page.IdPage, user.IdUser); err != nil {
		return err
	}
	s.notifySaved(ctx, space, page, user)
	return nil
}

func withTrailingNewline(text string) string {
	if text == "" || text[len(text)-1] == '\n' {
		return text
	}
	return text + "\n"
}

func (s *WikiService) siblingRank(ctx context.Context, idSpace int64, idParent, idSibling *int64) (string, error) {
	if idSibling == nil {
		return "", nil
	}
	sibling, err := s.pageRepo.LoadPage(ctx, *idSibling)
	if err != nil {
		return "", err
	}
	if sibling == nil || sibling.IdSpace != idSpace || sibling.DeletedAt != nil || !sameParent(sibling.IdParent, idParent) {
		return "", errs.ErrBadRequest
	}
	return sibling.Rank, nil
}

func sameParent(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func (s *WikiService) trashSpaceIds(ctx context.Context, user model.User, instance, project *model.WikiSpace) []int64 {
	idsSpace := []int64{project.IdSpace}
	if s.spaceAccess(ctx, user, instance).canEdit {
		idsSpace = append(idsSpace, instance.IdSpace)
	}
	return idsSpace
}
