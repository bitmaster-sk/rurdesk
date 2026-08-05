import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    Injector,
    OnInit,
    inject
} from '@angular/core';
import { takeUntilDestroyed, toObservable } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { combineLatest } from 'rxjs';
import { ProjectStore } from 'src/app/project/project.store';
import { AclStore } from 'src/app/project/store/acl.store';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { SavedViewApplyService } from '../../service/saved-view-apply.service';

@Component({
    selector: 'app-issue',
    templateUrl: './issue.page.html',
    styleUrls: ['./issue.page.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class IssuePage implements OnInit {
    private readonly projectStore = inject(ProjectStore);
    private readonly issueToolbarService = inject(IssueToolbarService);
    private readonly savedViewStore = inject(SavedViewStore);
    private readonly applyService = inject(SavedViewApplyService);
    private readonly route = inject(ActivatedRoute);
    private readonly toast = inject(UiToastService);
    private readonly i18n = inject(TranslateService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly injector = inject(Injector);
    protected readonly aclStore = inject(AclStore);

    public project$ = this.projectStore.project$;
    public toolbarTemplate = this.issueToolbarService.toolbarTemplate;

    public ngOnInit(): void {
        this.loadSavedViews();
        this.onViewQueryParamChange();
    }

    private loadSavedViews(): void {
        this.project$.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(project => {
            if (project) {
                this.savedViewStore.load(project.idProject);
            }
        });
    }

    private onViewQueryParamChange(): void {
        combineLatest([
            toObservable(this.savedViewStore.idLoadedProject, { injector: this.injector }),
            this.route.queryParamMap
        ])
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(([loadedProject, params]) => {
                if (loadedProject === null) {
                    return; // the id means nothing until the views are in
                }
                const raw = params.get('view');
                if (raw === null) {
                    this.savedViewStore.clearApplied();
                    return;
                }
                this.applyDeepLinkedView(Number(raw));
            });
    }

    private applyDeepLinkedView(idSavedView: number): void {
        // An apply writes `?view=` itself, so this guard stops the second apply.
        if (this.savedViewStore.idAppliedView() === idSavedView) {
            return;
        }
        const view = this.savedViewStore
            .views()
            .find(candidate => candidate.idSavedView === idSavedView);
        if (!view) {
            this.toast.show({
                severity: 'info',
                detail: this.i18n.instant('ISSUE.VIEWS.NOT_FOUND')
            });
            this.applyService.markUrl(null);
            return;
        }
        this.applyService.apply(view, view.idProject);
    }
}
