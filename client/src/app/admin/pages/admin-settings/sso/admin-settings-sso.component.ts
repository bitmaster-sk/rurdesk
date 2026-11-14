import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    computed,
    inject,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable, finalize, switchMap, tap } from 'rxjs';
import { AdminSsoApi } from '../../../api/admin-sso.api.service';
import { SSO_DOCS_URL } from '../../../constants/sso.constants';
import {
    InsertSsoMappingReq,
    SaveSsoDraftReq,
    SsoGroupMapping,
    SsoState,
    SsoTestResult,
    UpdateSsoRulesReq
} from '../../../model/admin-sso.model';
import { TeamApi } from 'src/app/team/api/team.api.service';
import { Team } from 'src/app/team/model/team.model';
import { SsoProtocol } from '../../../model/sso-protocol.enum';
import { SsoRedirect } from '../../../service/sso-redirect.service';

@Component({
    selector: 'app-admin-settings-sso',
    templateUrl: './admin-settings-sso.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminSettingsSsoComponent implements OnInit {
    private readonly api = inject(AdminSsoApi);
    private readonly teamApi = inject(TeamApi);
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly redirect = inject(SsoRedirect);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly docsUrl = SSO_DOCS_URL;
    protected readonly state = signal<SsoState | null>(null);
    protected readonly isUnavailable = signal(false);
    protected readonly testResult = signal<SsoTestResult | null>(null);
    protected readonly isBusy = signal(false);
    protected readonly mappings = signal<SsoGroupMapping[]>([]);
    protected readonly teams = signal<Team[]>([]);
    protected readonly selectedProtocol = signal<SsoProtocol | null>(null);
    protected readonly protocol = computed(
        () => this.selectedProtocol() ?? this.state()?.draft?.protocol ?? SsoProtocol.Oidc
    );
    protected readonly revealedScimToken = signal<string | null>(null);
    protected readonly suggestedGroups = computed(() => this.testResult()?.groups ?? []);

    protected readonly canEdit = computed(() => {
        const state = this.state();
        return !!state && state.isLicensed && state.hasEncryptionKey;
    });
    protected readonly canTest = computed(
        () => this.canEdit() && !!this.state()?.isPublicBaseUrlValid
    );
    protected readonly canActivate = computed(
        () => this.canTest() && !!this.state()?.isDraftTested
    );

    public ngOnInit(): void {
        this.api
            .load$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: state => this.state.set(state),
                error: () => this.isUnavailable.set(true)
            });
        this.api
            .loadMappings$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({ next: mappings => this.mappings.set(mappings), error: () => undefined });
        this.teamApi
            .load$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(teams => this.teams.set(teams));
        const idTest = this.route.snapshot.queryParamMap.get('test');
        if (idTest) {
            this.api
                .loadTestResult$(idTest)
                .pipe(takeUntilDestroyed(this.destroyRef))
                .subscribe(result => this.testResult.set(result));
            void this.router.navigate([], {
                relativeTo: this.route,
                queryParams: { test: null },
                queryParamsHandling: 'merge',
                replaceUrl: true
            });
        }
    }

    protected onSaveDraft(req: SaveSsoDraftReq): void {
        this.run(this.api.updateDraft$(req));
    }

    protected onSaveAndTest(req: SaveSsoDraftReq): void {
        this.isBusy.set(true);
        this.api
            .updateDraft$(req)
            .pipe(
                tap(state => this.state.set(state)),
                switchMap(() => this.api.startTest$()),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe({
                next: res => this.redirect.go(res.url),
                error: () => this.isBusy.set(false)
            });
    }

    protected onActivate(): void {
        this.run(this.api.activate$());
    }

    protected onDisable(): void {
        this.run(this.api.disable$());
    }

    protected onRegenerateSpCertificate(): void {
        this.run(this.api.regenerateSpCertificate$());
    }

    protected onSetEnforced(isEnforced: boolean): void {
        this.run(this.api.setEnforced$({ isEnforced }));
    }

    protected onSetScimEnabled(isEnabled: boolean): void {
        this.run(this.api.setScimEnabled$({ isEnabled }));
    }

    protected onGenerateScimToken(): void {
        this.isBusy.set(true);
        this.api
            .generateScimToken$()
            .pipe(
                tap(res => this.revealedScimToken.set(res.token)),
                switchMap(() => this.api.load$()),
                finalize(() => this.isBusy.set(false)),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(state => this.state.set(state));
    }

    protected onDeleteScimToken(): void {
        this.revealedScimToken.set(null);
        this.run(this.api.deleteScimToken$());
    }

    protected onSaveRules(req: UpdateSsoRulesReq): void {
        this.run(this.api.updateRules$(req));
    }

    protected onAddMapping(req: InsertSsoMappingReq): void {
        this.runMappings(this.api.insertMapping$(req));
    }

    protected onRemoveMapping(idMapping: number): void {
        this.runMappings(this.api.deleteMapping$(idMapping));
    }

    private runMappings(request$: Observable<SsoGroupMapping[]>): void {
        this.isBusy.set(true);
        request$
            .pipe(
                finalize(() => this.isBusy.set(false)),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(mappings => this.mappings.set(mappings));
    }

    private run(request$: Observable<SsoState>): void {
        this.isBusy.set(true);
        request$
            .pipe(
                finalize(() => this.isBusy.set(false)),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(state => this.state.set(state));
    }
}
