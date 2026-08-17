import { Injectable, inject, signal } from '@angular/core';
import { AppSettings, SettingsApi } from './settings.api.service';

const FALLBACK: AppSettings = {
    tablePageSize: 50,
    kanbanPageSize: 20,
    ganttBacklogPageSize: 30
};

@Injectable({ providedIn: 'root' })
export class SettingsStore {
    // Constructor injection (not inject()) keeps the store unit-testable with a fake api.
    constructor(private readonly api: SettingsApi) {}

    private readonly settings = signal<AppSettings>(FALLBACK);

    public readonly tablePageSize = (): number => this.settings().tablePageSize;
    public readonly kanbanPageSize = (): number => this.settings().kanbanPageSize;
    public readonly ganttBacklogPageSize = (): number => this.settings().ganttBacklogPageSize;

    public load(): void {
        this.api.getSettings$().subscribe(settings => this.settings.set(settings));
    }
}
