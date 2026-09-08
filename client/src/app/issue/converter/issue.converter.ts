import { HttpParams } from '@angular/common/http';
import { IssuesFilter } from '../components/filter/issue-filter.entity';
import { Issue } from '../model/issue.model';

export abstract class IssueConverter {
    public static toParams(filter: IssuesFilter): HttpParams {
        let params = new HttpParams();
        if (filter.orderColumn) {
            params = params.set('orderColumn', filter.orderColumn);
        }
        if (filter.orderDirection) {
            params = params.set('orderDirection', filter.orderDirection);
        }
        if (filter.idsSeverity?.length) {
            params = params.set('idsSeverity', filter.idsSeverity.join(','));
        }
        if (filter.idsIssueType?.length) {
            params = params.set('idsIssueType', filter.idsIssueType.join(','));
        }
        if (filter.idsState?.length) {
            params = params.set('idsState', filter.idsState.join(','));
        }
        if (filter.idsAssignedTo?.length) {
            params = params.set('idsAssignedTo', filter.idsAssignedTo.join(','));
        }
        if (filter.idsIssuePublic?.length) {
            params = params.set('idsIssuePublic', filter.idsIssuePublic.join(','));
        }
        // A window wins over the absolute pair — sending both would make the URL lie.
        if (filter.createAtWithin) {
            params = params.set('createAtWithin', filter.createAtWithin);
        } else {
            if (filter.createAtFrom) {
                params = params.set('createAtFrom', filter.createAtFrom.toISOString());
            }
            if (filter.createAtTo) {
                params = params.set('createAtTo', filter.createAtTo.toISOString());
            }
        }
        if (filter.updateAtWithin) {
            params = params.set('updateAtWithin', filter.updateAtWithin);
        } else {
            if (filter.updateAtFrom) {
                params = params.set('updateAtFrom', filter.updateAtFrom.toISOString());
            }
            if (filter.updateAtTo) {
                params = params.set('updateAtTo', filter.updateAtTo.toISOString());
            }
        }
        if (filter.scheduledAtFrom) {
            params = params.set('scheduledAtFrom', filter.scheduledAtFrom.toISOString());
        }
        if (filter.scheduledAtTo) {
            params = params.set('scheduledAtTo', filter.scheduledAtTo.toISOString());
        }
        if (filter.scheduledAtUnset) {
            params = params.set('scheduledAtUnset', 'true');
        }
        if (filter.assignedToNull) {
            params = params.set('assignedToNull', 'true');
        }
        if (filter.title) {
            params = params.set('title', filter.title);
        }
        if (filter.sprintUnset) {
            params = params.set('sprintUnset', 'true');
        } else if (filter.idSprint != null) {
            params = params.set('idSprint', String(filter.idSprint));
        }
        params = params.set('stateUnset', filter.stateUnset ? 'true' : 'false');
        params = params.set('severityUnset', filter.severityUnset ? 'true' : 'false');
        params = params.set('issueTypeUnset', filter.issueTypeUnset ? 'true' : 'false');
        params = params.set('assignedToUnset', filter.assignedToUnset ? 'true' : 'false');
        return params;
    }

    public static toIssue(issue: Issue): Issue {
        return {
            ...issue,
            createAt: issue.createAt ? new Date(issue.createAt) : undefined,
            updateAt: issue.updateAt ? new Date(issue.updateAt) : undefined,
            scheduledAt: issue.scheduledAt ? new Date(issue.scheduledAt) : null
        };
    }
}
