import { Injectable, inject, signal } from '@angular/core';
import { AppSettings, SettingsApi } from './settings.api.service';

const FALLBACK: AppSettings = {
    tablePageSize: 50,
    kanbanPageSize: 20,
    ganttBacklogPageSize: 30,
    userApiKeyLimit: 10
};

@Injectable({ providedIn: 'root' })
export class SettingsStore {
    // Injected as a field, so specs must build the store through an Injector rather than `new`.
    private readonly api = inject(SettingsApi);

    private readonly settings = signal<AppSettings>(FALLBACK);

    public readonly tablePageSize = (): number => this.settings().tablePageSize;
    public readonly kanbanPageSize = (): number => this.settings().kanbanPageSize;
    public readonly ganttBacklogPageSize = (): number => this.settings().ganttBacklogPageSize;
    public readonly userApiKeyLimit = (): number => this.settings().userApiKeyLimit;

    public load(): void {
        this.api.getSettings$().subscribe(settings => this.settings.set(settings));
    }
}
