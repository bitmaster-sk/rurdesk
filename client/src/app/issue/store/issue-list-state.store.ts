import { Injectable, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
    Event,
    NavigationCancel,
    NavigationEnd,
    NavigationError,
    NavigationStart,
    Router,
    RoutesRecognized
} from '@angular/router';
import { SavedViewConfigConverter } from 'src/app/project/model/saved-view.converter';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { IssuesFilter } from '../components/filter/issue-filter.entity';
import { IssueFilterStore } from '../components/filter/issue-filter.store';
import { IssueViewMode } from '../constants/issue-view-modes.enum';
import { IssueListFilterConverter } from '../converter/issue-list-filter.converter';
import { IssueListLocation } from '../entity/issue-list-location.entity';
import { IssueListPosition } from '../entity/issue-list-position.entity';

@Injectable()
export class IssueListStateStore {
    private readonly router = inject(Router);
    private readonly filterStore = inject(IssueFilterStore);
    private readonly savedViewStore = inject(SavedViewStore);

    private readonly positions = new Map<string, IssueListPosition>();

    private readPosition: (() => IssueListPosition) | null = null;

    private targetUrl: string;
    private settledUrl: string;
    private isHistoryNavigation: boolean;

    public constructor() {
        const navigation = this.router.currentNavigation();
        this.settledUrl = this.router.url;
        this.targetUrl = navigation
            ? this.router.serializeUrl(navigation.finalUrl ?? navigation.extractedUrl)
            : this.router.url;
        this.isHistoryNavigation = navigation?.trigger === 'popstate';

        this.router.events.pipe(takeUntilDestroyed()).subscribe(event => this.onRouterEvent(event));
        this.filterStore.changedFilter$
            .pipe(takeUntilDestroyed())
            .subscribe(filter => this.saveFilter(filter));
    }

    public restoreFilter(): IssuesFilter | null {
        const location = this.locate(this.targetUrl);
        if (!location) {
            return null;
        }
        const filter =
            location.idSavedView === null
                ? IssueListFilterConverter.toFilter(localStorage.getItem(this.storageKey(location)))
                : this.savedViewFilter(location);
        return filter?.idProject === location.idProject ? filter : null;
    }

    public forgetFilter(): void {
        const location = this.locate(this.targetUrl);
        if (!location) {
            return;
        }
        localStorage.removeItem(this.storageKey(location));
    }

    public registerPosition(read: () => IssueListPosition): void {
        this.readPosition = read;
    }

    public unregisterPosition(read: () => IssueListPosition): void {
        if (this.readPosition === read) {
            this.readPosition = null;
        }
    }

    public restorePosition(): IssueListPosition | null {
        const location = this.locate(this.targetUrl);
        if (!location || !this.isHistoryNavigation) {
            return null;
        }
        const key = this.key(location);
        const position = this.positions.get(key) ?? null;
        this.positions.delete(key);
        return position;
    }

    private saveFilter(filter: IssuesFilter): void {
        const location = this.locate(this.targetUrl);
        if (location?.idProject !== filter.idProject || location.idSavedView !== null) {
            return;
        }
        const preference: IssuesFilter = { ...filter, idSprint: undefined, sprintUnset: undefined };
        localStorage.setItem(
            this.storageKey(location),
            IssueListFilterConverter.toStorage(preference)
        );
    }

    private savedViewFilter(location: IssueListLocation): IssuesFilter | null {
        const view = this.savedViewStore
            .views()
            .find(
                candidate =>
                    candidate.idSavedView === location.idSavedView &&
                    candidate.idProject === location.idProject
            );
        return view
            ? { ...SavedViewConfigConverter.toFilter(view.config), idProject: location.idProject }
            : null;
    }

    private onRouterEvent(event: Event): void {
        if (event instanceof NavigationStart) {
            this.capturePosition();
            this.targetUrl = event.url;
            this.isHistoryNavigation = event.navigationTrigger === 'popstate';
        } else if (event instanceof RoutesRecognized) {
            this.targetUrl = event.urlAfterRedirects;
        } else if (event instanceof NavigationEnd) {
            this.targetUrl = event.urlAfterRedirects;
            this.settledUrl = event.urlAfterRedirects;
        } else if (event instanceof NavigationCancel || event instanceof NavigationError) {
            this.targetUrl = this.settledUrl;
        }
    }

    // Runs on NavigationStart, before the router detaches the view, while its scroll
    // container still reports the real offsets.
    private capturePosition(): void {
        const location = this.locate(this.settledUrl);
        if (location && this.readPosition) {
            this.positions.set(this.key(location), this.readPosition());
        }
    }

    private locate(url: string): IssueListLocation | null {
        const match = /\/project\/(\d+)\/issue\/view\/([^/?#]+)/.exec(url);
        const modes: string[] = Object.values(IssueViewMode);
        if (!match || !modes.includes(match[2])) {
            return null;
        }
        const rawView = this.router.parseUrl(url).queryParamMap.get('view');
        return {
            idProject: Number(match[1]),
            mode: match[2] as IssueViewMode,
            idSavedView: rawView === null ? null : Number(rawView)
        };
    }

    private key(location: IssueListLocation): string {
        return `${location.idProject}:${location.mode}:${location.idSavedView ?? ''}`;
    }

    private storageKey(location: IssueListLocation): string {
        return `issue-list-filter:${location.idProject}:${location.mode}`;
    }
}
