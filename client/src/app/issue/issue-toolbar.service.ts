import { Injectable, signal, TemplateRef } from '@angular/core';

@Injectable()
export class IssueToolbarService {
    public readonly toolbarTemplate = signal<TemplateRef<unknown> | null>(null);

    public register(template: TemplateRef<unknown>): void {
        this.toolbarTemplate.set(template);
    }

    public clear(): void {
        this.toolbarTemplate.set(null);
    }
}
