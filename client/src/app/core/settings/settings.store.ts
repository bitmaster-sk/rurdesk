import { Injectable, inject, signal } from '@angular/core';
import { AppSettings } from './model/app-settings.model';
import { SettingsApi } from './settings.api.service';

// Mirrors the server-side defaults in api/internal/constants/app_settings.go.
const FALLBACK: AppSettings = {
    tablePageSize: 50,
    kanbanPageSize: 20,
    ganttBacklogPageSize: 30,
    sprintVelocityLimit: 10,
    userApiKeyLimit: 10,
    isAgentThinkingPersisted: true,
    agentThinkingMaxKb: 1024
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
        this.api.load$().subscribe(settings => this.settings.set(settings));
    }
}
