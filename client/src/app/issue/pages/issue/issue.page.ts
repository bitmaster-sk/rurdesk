import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { ProjectStore } from 'src/app/project/project.store';
import { AclStore } from 'src/app/project/store/acl.store';
import { IssueToolbarService } from '../../issue-toolbar.service';

@Component({
    selector: 'app-issue',
    templateUrl: './issue.page.html',
    styleUrls: ['./issue.page.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class IssuePage {
    private readonly projectStore = inject(ProjectStore);
    private readonly issueToolbarService = inject(IssueToolbarService);
    protected readonly aclStore = inject(AclStore);

    public project$ = this.projectStore.project$;
    public toolbarTemplate = this.issueToolbarService.toolbarTemplate;
}
